package ui

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lxn/walk"
	d "github.com/lxn/walk/declarative"
	"github.com/lxn/win"

	"usb-suspend-watch/internal/logs"
	"usb-suspend-watch/internal/model"
	"usb-suspend-watch/internal/monitor"
	"usb-suspend-watch/internal/netstats"
	"usb-suspend-watch/internal/platform"
	"usb-suspend-watch/internal/usbpcap"
)

type Config struct {
	Version string
}

type app struct {
	cfg Config

	mw                 *walk.MainWindow
	refreshButton      *walk.PushButton
	startETWButton     *walk.PushButton
	stopETWButton      *walk.PushButton
	startUSBPcapButton *walk.PushButton
	stopUSBPcapButton  *walk.PushButton
	openLogsButton     *walk.PushButton
	exportButton       *walk.PushButton
	languageLabel      *walk.Label
	languageCombo      *walk.ComboBox
	statusGroup        *walk.GroupBox
	deviceView         *walk.TableView
	groupView          *walk.TableView
	usbChangeView      *walk.TableView
	eventView          *walk.TableView
	sequenceView       *walk.TableView
	details            *walk.TextEdit
	rawDetails         *walk.TextEdit
	devicesLabel       *walk.Label
	groupsLabel        *walk.Label
	usbChangesLabel    *walk.Label
	timelineLabel      *walk.Label
	sequenceLabel      *walk.Label
	detailsLabel       *walk.Label
	rawDetailsLabel    *walk.Label
	typeLabel          *walk.Label
	targetLabel        *walk.Label
	confidenceLabel    *walk.Label
	levelLabel         *walk.Label
	statusLabel        *walk.Label
	summaryLabel       *walk.Label
	privilegeLabel     *walk.Label
	logPathLabel       *walk.Label
	eventTypeFilter    *walk.ComboBox
	targetFilter       *walk.ComboBox
	confidenceFilter   *walk.ComboBox
	levelFilter        *walk.ComboBox
	eventSearch        *walk.LineEdit
	notifyIcon         *walk.NotifyIcon
	showAction         *walk.Action
	logAction          *walk.Action
	exitAction         *walk.Action
	icon               *walk.Icon

	devices        *deviceTableModel
	groups         *adapterGroupTableModel
	events         *eventTableModel
	usbChanges     *eventTableModel
	sequence       *eventTableModel
	poller         *monitor.Poller
	logger         *logs.EventLogger
	deviceLogger   *logs.JSONLWriter
	netLogger      *logs.JSONLWriter
	logDir         string
	logPrefix      string
	sessionStarted time.Time
	correlationID  string

	ctx        context.Context
	cancel     context.CancelFunc
	tailCancel context.CancelFunc

	wndProc    uintptr
	oldWndProc uintptr

	etwRunning        bool
	etwStopFile       string
	etwHelperAdmin    *bool
	usbpcapStarting   bool
	usbpcapRunning    bool
	usbpcapStartID    int
	usbpcapCmd        *exec.Cmd
	usbpcapOutputPath string
	detailedUntil     time.Time
	mu                sync.Mutex

	selectionChanging bool
	localizing        bool
	language          displayLanguage
	statusText        string
	watchedDevice     model.DeviceSnapshot
	hasWatchedDevice  bool
	watchFocusActive  bool
}

func Run(cfg Config) error {
	logDir, err := logs.ResolveLogDir()
	if err != nil {
		return err
	}
	sessionStarted := time.Now()
	correlationID := makeCorrelationID(sessionStarted)
	logPrefix := fmt.Sprintf("usb_watch_%s_%s", sessionStarted.Format("20060102_150405"), correlationID)
	logger, err := logs.NewEventLoggerAtPath(filepath.Join(logDir, logPrefix+"_events.jsonl"))
	if err != nil {
		return err
	}
	defer logger.Close()
	deviceLogger, err := logs.NewJSONLWriter(filepath.Join(logDir, logPrefix+"_device.jsonl"))
	if err != nil {
		return err
	}
	defer deviceLogger.Close()
	netLogger, err := logs.NewJSONLWriter(filepath.Join(logDir, logPrefix+"_net.jsonl"))
	if err != nil {
		return err
	}
	defer netLogger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	a := &app{
		cfg:            cfg,
		devices:        newDeviceTableModel(),
		groups:         newAdapterGroupTableModel(),
		events:         newEventTableModel(2000),
		usbChanges:     newEventTableModel(500),
		sequence:       newEventTableModel(200),
		poller:         monitor.NewPoller(2 * time.Second),
		logger:         logger,
		deviceLogger:   deviceLogger,
		netLogger:      netLogger,
		logDir:         logDir,
		logPrefix:      logPrefix,
		sessionStarted: sessionStarted,
		correlationID:  correlationID,
		ctx:            ctx,
		cancel:         cancel,
		language:       languageJapanese,
	}
	a.devices.onMonitorChanged = a.onDeviceMonitorChanged

	if err := a.createWindow(); err != nil {
		cancel()
		return err
	}
	defer func() {
		cancel()
		a.cleanup()
	}()

	go a.consumePollerEvents()

	a.poller.Prime()
	go a.poller.Run(ctx)
	go a.runSnapshotLogging(ctx)
	a.refreshDevices()
	a.addEvent(model.Event{
		Time:       a.sessionStarted,
		Type:       model.EventInfo,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceHigh,
		Message:    "USB Suspend Watch started",
		Raw: map[string]string{
			"session_started_at": a.sessionStarted.Format(time.RFC3339Nano),
			"correlation_id":     a.correlationID,
			"device_log":         a.deviceLogger.Path(),
			"events_log":         a.logger.Path(),
			"network_log":        a.netLogger.Path(),
		},
	}, true)

	a.updateStatus(statusSimpleMonitorRunning)
	a.mw.Run()
	return nil
}

func makeCorrelationID(t time.Time) string {
	var b [3]byte
	if _, err := crand.Read(b[:]); err == nil {
		return "s" + hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("s%06x", t.UnixNano()&0xffffff)
}

func (a *app) createWindow() error {
	var err error
	a.icon, err = buildIcon()
	if err != nil {
		return err
	}
	text := a.text()

	err = d.MainWindow{
		AssignTo: &a.mw,
		Title:    "USB Suspend Watch",
		Size:     d.Size{Width: 1180, Height: 760},
		MinSize:  d.Size{Width: 860, Height: 560},
		Icon:     a.icon,
		Layout:   d.VBox{Margins: d.Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
		OnSizeChanged: func() {
			if a.mw != nil && win.IsIconic(a.mw.Handle()) {
				a.hideToTray()
			}
		},
		Children: []d.Widget{
			d.Composite{
				Layout: d.HBox{MarginsZero: true},
				Children: []d.Widget{
					d.PushButton{AssignTo: &a.refreshButton, Text: text.refreshButton, OnClicked: a.refreshDevices},
					d.PushButton{AssignTo: &a.startETWButton, Text: text.startETWButton, OnClicked: a.startETW},
					d.PushButton{AssignTo: &a.stopETWButton, Text: text.stopETWButton, OnClicked: a.stopETW},
					d.PushButton{AssignTo: &a.startUSBPcapButton, Text: text.startUSBPcapButton, OnClicked: a.startUSBPcap},
					d.PushButton{AssignTo: &a.stopUSBPcapButton, Text: text.stopUSBPcapButton, OnClicked: a.stopUSBPcap},
					d.PushButton{AssignTo: &a.openLogsButton, Text: text.openLogsButton, OnClicked: a.openLogFolder},
					d.PushButton{AssignTo: &a.exportButton, Text: text.exportVisibleButton, OnClicked: a.exportVisibleLog},
					d.Label{AssignTo: &a.languageLabel, Text: text.languageLabel},
					d.ComboBox{
						AssignTo:              &a.languageCombo,
						Model:                 text.languageOptions,
						CurrentIndex:          a.language.index(),
						MinSize:               d.Size{Width: 120},
						OnCurrentIndexChanged: a.onLanguageChanged,
					},
				},
			},
			d.GroupBox{
				AssignTo: &a.statusGroup,
				Title:    text.monitoringStatusTitle,
				Layout:   d.Grid{Columns: 3, Margins: d.Margins{Left: 8, Top: 6, Right: 8, Bottom: 6}, Spacing: 6},
				Children: []d.Widget{
					d.Label{AssignTo: &a.statusLabel, Text: text.monitoringPrefix + ": starting", ColumnSpan: 2, EllipsisMode: d.EllipsisEnd},
					d.Label{AssignTo: &a.privilegeLabel, Text: text.privilegePrefix + ": " + text.privilegeChecking, EllipsisMode: d.EllipsisEnd},
					d.Label{AssignTo: &a.summaryLabel, Text: fmt.Sprintf(text.summaryFormat, 0, 0, 0, 0, 0, 0), ColumnSpan: 2, EllipsisMode: d.EllipsisEnd},
					d.Label{AssignTo: &a.logPathLabel, Text: text.logPrefix + ": " + a.logDir, EllipsisMode: d.EllipsisPath},
				},
			},
			d.HSplitter{
				Children: []d.Widget{
					d.Composite{
						Layout: d.VBox{MarginsZero: true},
						Children: []d.Widget{
							d.Label{AssignTo: &a.devicesLabel, Text: text.connectedDevicesTitle},
							d.TableView{
								AssignTo:         &a.deviceView,
								AlternatingRowBG: true,
								CheckBoxes:       true,
								ColumnsOrderable: true,
								Columns: []d.TableViewColumn{
									{Title: text.deviceColumnTitles[0], Width: 250},
									{Title: text.deviceColumnTitles[1], Width: 170},
									{Title: text.deviceColumnTitles[2], Width: 170},
									{Title: text.deviceColumnTitles[3], Width: 110},
									{Title: text.deviceColumnTitles[4], Width: 70},
									{Title: text.deviceColumnTitles[5], Width: 90},
									{Title: text.deviceColumnTitles[6], Width: 85},
									{Title: text.deviceColumnTitles[7], Width: 160},
									{Title: text.deviceColumnTitles[8], Width: 115},
									{Title: text.deviceColumnTitles[9], Width: 260},
									{Title: text.deviceColumnTitles[10], Width: 240},
									{Title: text.deviceColumnTitles[11], Width: 180},
									{Title: text.deviceColumnTitles[12], Width: 90},
									{Title: text.deviceColumnTitles[13], Width: 240},
									{Title: text.deviceColumnTitles[14], Width: 100},
									{Title: text.deviceColumnTitles[15], Width: 220},
									{Title: text.deviceColumnTitles[16], Width: 70},
									{Title: text.deviceColumnTitles[17], Width: 70},
									{Title: text.deviceColumnTitles[18], Width: 260},
									{Title: text.deviceColumnTitles[19], Width: 220},
									{Title: text.deviceColumnTitles[20], Width: 180},
								},
								Model: a.devices,
								OnSelectedIndexesChanged: func() {
									a.onDeviceSelectionChanged()
								},
								OnItemActivated: func() {
									a.openSelectedDeviceDetailsWindow()
								},
							},
							d.Label{AssignTo: &a.groupsLabel, Text: text.adapterGroupsTitle},
							d.TableView{
								AssignTo:         &a.groupView,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								MaxSize:          d.Size{Height: 130},
								Columns: []d.TableViewColumn{
									{Title: text.groupColumnTitles[0], Width: 170},
									{Title: text.groupColumnTitles[1], Width: 95},
									{Title: text.groupColumnTitles[2], Width: 210},
									{Title: text.groupColumnTitles[3], Width: 210},
									{Title: text.groupColumnTitles[4], Width: 190},
								},
								Model: a.groups,
							},
							d.Label{AssignTo: &a.usbChangesLabel, Text: text.usbChangesTitle},
							d.TableView{
								AssignTo:         &a.usbChangeView,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								MaxSize:          d.Size{Height: 190},
								Columns: []d.TableViewColumn{
									{Title: text.eventColumnTitles[0], Width: 70},
									{Title: text.eventColumnTitles[1], Width: 135},
									{Title: text.eventColumnTitles[2], Width: 120},
									{Title: text.eventColumnTitles[3], Width: 80},
									{Title: text.eventColumnTitles[4], Width: 95},
									{Title: text.eventColumnTitles[5], Width: 190},
									{Title: text.eventColumnTitles[6], Width: 260},
								},
								Model: a.usbChanges,
								OnSelectedIndexesChanged: func() {
									a.onUSBChangeSelectionChanged()
								},
							},
						},
					},
					d.VSplitter{
						Children: []d.Widget{
							d.Composite{
								Layout: d.VBox{MarginsZero: true},
								Children: []d.Widget{
									d.Label{AssignTo: &a.timelineLabel, Text: text.timelineTitle},
									d.Composite{
										Layout: d.VBox{MarginsZero: true, Spacing: 3},
										Children: []d.Widget{
											d.Composite{
												Layout: d.HBox{MarginsZero: true},
												Children: []d.Widget{
													d.Label{AssignTo: &a.typeLabel, Text: text.typeLabel},
													d.ComboBox{
														AssignTo:              &a.eventTypeFilter,
														Model:                 text.typeOptions,
														CurrentIndex:          0,
														StretchFactor:         1,
														OnCurrentIndexChanged: a.applyEventFilters,
													},
													d.Label{AssignTo: &a.targetLabel, Text: text.targetLabel},
													d.ComboBox{
														AssignTo:              &a.targetFilter,
														Model:                 text.targetOptions,
														CurrentIndex:          1,
														StretchFactor:         1,
														OnCurrentIndexChanged: a.applyTargetFilter,
													},
													d.Label{AssignTo: &a.confidenceLabel, Text: text.confidenceLabel},
													d.ComboBox{
														AssignTo:              &a.confidenceFilter,
														Model:                 text.confidenceOptions,
														CurrentIndex:          0,
														StretchFactor:         1,
														OnCurrentIndexChanged: a.applyEventFilters,
													},
													d.Label{AssignTo: &a.levelLabel, Text: text.levelLabel},
													d.ComboBox{
														AssignTo:              &a.levelFilter,
														Model:                 text.levelOptions,
														CurrentIndex:          0,
														StretchFactor:         1,
														OnCurrentIndexChanged: a.applyEventFilters,
													},
												},
											},
											d.LineEdit{
												AssignTo:      &a.eventSearch,
												CueBanner:     text.searchCue,
												StretchFactor: 1,
												OnTextChanged: a.applyEventFilters,
											},
										},
									},
									d.TableView{
										AssignTo:         &a.eventView,
										AlternatingRowBG: true,
										ColumnsOrderable: true,
										Columns: []d.TableViewColumn{
											{Title: text.eventColumnTitles[0], Width: 95},
											{Title: text.eventColumnTitles[1], Width: 150},
											{Title: text.eventColumnTitles[2], Width: 125},
											{Title: text.eventColumnTitles[3], Width: 90},
											{Title: text.eventColumnTitles[4], Width: 120},
											{Title: text.eventColumnTitles[5], Width: 240},
											{Title: text.eventColumnTitles[6], Width: 340},
										},
										Model: a.events,
										OnSelectedIndexesChanged: func() {
											a.onEventSelectionChanged()
										},
									},
								},
							},
							d.Composite{
								Layout: d.VBox{MarginsZero: true},
								Children: []d.Widget{
									d.Label{AssignTo: &a.sequenceLabel, Text: text.selectedSequenceTitle},
									d.TableView{
										AssignTo:         &a.sequenceView,
										AlternatingRowBG: true,
										ColumnsOrderable: true,
										MaxSize:          d.Size{Height: 150},
										Columns: []d.TableViewColumn{
											{Title: text.eventColumnTitles[0], Width: 90},
											{Title: text.eventColumnTitles[1], Width: 145},
											{Title: text.eventColumnTitles[2], Width: 125},
											{Title: text.eventColumnTitles[3], Width: 85},
											{Title: text.eventColumnTitles[4], Width: 110},
											{Title: text.eventColumnTitles[5], Width: 210},
											{Title: text.eventColumnTitles[6], Width: 310},
										},
										Model: a.sequence,
									},
									d.HSplitter{
										Children: []d.Widget{
											d.Composite{
												Layout: d.VBox{MarginsZero: true},
												Children: []d.Widget{
													d.Label{AssignTo: &a.detailsLabel, Text: text.diagnosticDetailsTitle},
													d.TextEdit{
														AssignTo: &a.details,
														ReadOnly: true,
														VScroll:  true,
														HScroll:  true,
														Text:     text.emptyDetails,
													},
												},
											},
											d.Composite{
												Layout: d.VBox{MarginsZero: true},
												Children: []d.Widget{
													d.Label{AssignTo: &a.rawDetailsLabel, Text: text.rawDetailsTitle},
													d.TextEdit{
														AssignTo: &a.rawDetails,
														ReadOnly: true,
														VScroll:  true,
														HScroll:  true,
														Text:     "{}",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}.Create()
	if err != nil {
		return err
	}

	a.setupNotifyIcon()
	a.installWndProc()
	a.applyLanguage()
	return nil
}

func (a *app) setupNotifyIcon() {
	text := a.text()
	ni, err := walk.NewNotifyIcon(a.mw)
	if err != nil {
		return
	}
	a.notifyIcon = ni
	_ = ni.SetIcon(a.icon)
	_ = ni.SetToolTip("USB Suspend Watch")
	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.showFromTray()
		}
	})
	showAction := walk.NewAction()
	a.showAction = showAction
	_ = showAction.SetText(text.trayShow)
	showAction.Triggered().Attach(a.showFromTray)
	logAction := walk.NewAction()
	a.logAction = logAction
	_ = logAction.SetText(text.trayOpenLogs)
	logAction.Triggered().Attach(a.openLogFolder)
	exitAction := walk.NewAction()
	a.exitAction = exitAction
	_ = exitAction.SetText(text.trayExit)
	exitAction.Triggered().Attach(func() { walk.App().Exit(0) })
	_ = ni.ContextMenu().Actions().Add(showAction)
	_ = ni.ContextMenu().Actions().Add(logAction)
	_ = ni.ContextMenu().Actions().Add(exitAction)
	_ = ni.SetVisible(true)
}

func (a *app) installWndProc() {
	a.wndProc = syscall.NewCallback(func(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
		if msg == win.WM_DEVICECHANGE {
			a.handleDeviceChange(wParam)
		}
		if msg == win.WM_POWERBROADCAST {
			a.handlePowerBroadcast(wParam)
		}
		if a.oldWndProc == 0 {
			return win.DefWindowProc(hwnd, msg, wParam, lParam)
		}
		return win.CallWindowProc(a.oldWndProc, hwnd, msg, wParam, lParam)
	})
	a.oldWndProc = win.SetWindowLongPtr(a.mw.Handle(), win.GWLP_WNDPROC, a.wndProc)
}

func (a *app) cleanup() {
	a.stopUSBPcapProcess(false)
	a.stopETW()
	if a.oldWndProc != 0 && a.mw != nil {
		win.SetWindowLongPtr(a.mw.Handle(), win.GWLP_WNDPROC, a.oldWndProc)
	}
	if a.notifyIcon != nil {
		a.notifyIcon.Dispose()
	}
	if a.icon != nil {
		a.icon.Dispose()
	}
}

func (a *app) consumePollerEvents() {
	for event := range a.poller.Events() {
		ev := event
		a.mw.Synchronize(func() {
			a.addEvent(ev, true)
			a.refreshDevices()
		})
	}
}

func (a *app) refreshDevices() {
	target, hasTarget := a.currentDeviceSelectionTarget()
	devices := a.poller.Snapshot()
	for i := range devices {
		devices[i].CorrelationID = a.correlationID
	}
	devices = filterDevicesForTarget(devices, a.currentTargetFilterIndex())
	sort.Slice(devices, func(i, j int) bool {
		return strings.ToLower(devices[i].DisplayName()) < strings.ToLower(devices[j].DisplayName())
	})
	a.devices.Set(devices)
	if a.groups != nil {
		a.groups.Set(devices)
	}
	if hasTarget {
		a.restoreDeviceSelection(target)
	}
	a.updateDetails()
	a.updateStatus(statusSimpleMonitorRunning)
}

func (a *app) runSnapshotLogging(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastDeviceLog time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			interval := 5 * time.Second
			if a.isDetailedLoggingActive(now) {
				interval = 1 * time.Second
			}
			if !lastDeviceLog.IsZero() && now.Sub(lastDeviceLog) < interval {
				continue
			}
			lastDeviceLog = now
			a.writeDeviceSnapshots(now)
		}
	}
}

func (a *app) isDetailedLoggingActive(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.detailedUntil.IsZero() && now.Before(a.detailedUntil)
}

func (a *app) writeDeviceSnapshots(now time.Time) {
	devices := a.poller.Snapshot()
	shouldCaptureNet := false
	for _, device := range devices {
		device.CorrelationID = a.correlationID
		_ = a.deviceLogger.Append(deviceSnapshotLogRecord(now, a.correlationID, device))
		if netstats.IsNetworkDevice(device) {
			shouldCaptureNet = true
		}
	}
	if !shouldCaptureNet {
		return
	}
	stats, err := netstats.Capture(a.ctx, a.correlationID, now)
	if err != nil {
		_ = a.netLogger.Append(map[string]string{
			"time":           now.Format(time.RFC3339Nano),
			"correlation_id": a.correlationID,
			"net_error":      err.Error(),
		})
		return
	}
	for _, stat := range stats {
		_ = a.netLogger.Append(stat)
	}
}

func deviceSnapshotLogRecord(t time.Time, correlationID string, d model.DeviceSnapshot) map[string]interface{} {
	return map[string]interface{}{
		"Timestamp":               t.Format(time.RFC3339Nano),
		"CorrelationID":           correlationID,
		"Name":                    d.DisplayName(),
		"InstanceId":              d.InstanceID,
		"ParentInstanceId":        d.ParentInstanceID,
		"ContainerId":             d.ContainerID,
		"LocationPath":            strings.Join(d.LocationPaths, " | "),
		"Status":                  d.Status,
		"StatusFlags":             d.StatusFlags,
		"StatusFlagNames":         d.StatusFlagNames,
		"ProblemCode":             d.ProblemCode,
		"ConfigManagerErrorCode":  d.ConfigManagerErrorCode,
		"Enumerator":              d.Enumerator,
		"Service":                 d.Service,
		"Class":                   d.Class,
		"ClassGuid":               d.ClassGuid,
		"Driver":                  d.Driver,
		"VID":                     d.VID,
		"PID":                     d.PID,
		"PD_MostRecentPowerState": d.PowerData.MostRecentPowerState,
		"PD_Capabilities":         fmt.Sprintf("0x%08X", d.PowerData.Capabilities),
		"PD_D1Latency":            d.PowerData.D1Latency,
		"PD_D2Latency":            d.PowerData.D2Latency,
		"PD_D3Latency":            d.PowerData.D3Latency,
		"PD_PowerStateMapping":    d.PowerData.PowerStateMapping,
		"PD_DeepestSystemWake":    d.PowerData.DeepestSystemWake,
		"PowerState":              d.PowerState,
		"ConnectionTime":          formatOptionalTime(d.ConnectedAt),
		"LastSeenTime":            formatOptionalTime(d.LastSeen),
		"ParentLowPowerChildD0":   d.ParentLowPowerChildD0,
		"USBPcapHints":            d.USBPcapHints,
	}
}

func (a *app) addEvent(event model.Event, persist bool) {
	if !a.isEventMonitored(event) {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	event = a.withSessionRaw(event)
	a.events.Add(event)
	if a.usbChanges != nil && isUSBChangeTimelineEvent(event) {
		a.usbChanges.Add(event)
	}
	if persist {
		_ = a.logger.Append(event)
	}
	if isDetailedLoggingTrigger(event) {
		a.triggerDetailedLogging(event)
	}
	if a.shouldAutoSelectLatestEvent() && a.eventView != nil && a.events.RowCount() > 0 && eventMatchesFilter(event, a.events.filter) {
		_ = a.eventView.SetSelectedIndexes([]int{a.events.RowCount() - 1})
	}
	a.updateSummary()
	if a.shouldRefreshWatchDetailsForEvent(event) {
		a.updateDetails()
	}
	a.notifyImportantEvent(event)
}

func isDetailedLoggingTrigger(event model.Event) bool {
	switch event.Type {
	case model.EventDStateTransition,
		model.EventPowerD0Exit,
		model.EventPowerD0Entry,
		model.EventSuspectSuspend,
		model.EventParentMismatch,
		model.EventProblemCode,
		model.EventStatusChanged,
		model.EventDeviceMissing,
		model.EventDeviceReenum,
		model.EventLastSeenStale,
		model.EventError:
		return true
	default:
		return false
	}
}

func (a *app) triggerDetailedLogging(trigger model.Event) {
	now := time.Now()
	a.mu.Lock()
	wasActive := !a.detailedUntil.IsZero() && now.Before(a.detailedUntil)
	a.detailedUntil = now.Add(60 * time.Second)
	a.mu.Unlock()
	if wasActive {
		return
	}
	raw := map[string]string{
		"correlation_id":      a.correlationID,
		"detailed_until":      a.detailedUntil.Format(time.RFC3339Nano),
		"trigger_type":        string(trigger.Type),
		"trigger_source":      string(trigger.Source),
		"trigger_device":      trigger.Device.DisplayName(),
		"trigger_instance_id": trigger.Device.InstanceID,
	}
	a.addEvent(model.Event{
		Time:       now,
		Type:       model.EventDetailedLogging,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceHigh,
		Device:     trigger.Device,
		Message:    "detailed 1-second logging started for 60 seconds: " + string(trigger.Type),
		Raw:        raw,
	}, true)
}

func (a *app) onDeviceSelectionChanged() {
	if a.selectionChanging {
		return
	}
	if idx := selectedIndex(a.deviceView); idx >= 0 {
		if device, ok := a.devices.Item(idx); ok {
			a.watchedDevice = device
			a.hasWatchedDevice = true
			a.watchFocusActive = true
		}
	}
	if selectedIndex(a.deviceView) >= 0 && a.eventView != nil {
		a.selectionChanging = true
		_ = a.eventView.SetSelectedIndexes([]int{})
		if a.usbChangeView != nil {
			_ = a.usbChangeView.SetSelectedIndexes([]int{})
		}
		a.selectionChanging = false
	}
	a.updateDetails()
}

func (a *app) onEventSelectionChanged() {
	if a.selectionChanging {
		return
	}
	if selectedIndex(a.eventView) >= 0 && a.deviceView != nil {
		a.watchFocusActive = false
		a.selectionChanging = true
		_ = a.deviceView.SetSelectedIndexes([]int{})
		if a.usbChangeView != nil {
			_ = a.usbChangeView.SetSelectedIndexes([]int{})
		}
		a.selectionChanging = false
	}
	a.updateDetails()
}

func (a *app) onUSBChangeSelectionChanged() {
	if a.selectionChanging {
		return
	}
	if selectedIndex(a.usbChangeView) >= 0 {
		a.watchFocusActive = false
		a.selectionChanging = true
		if a.deviceView != nil {
			_ = a.deviceView.SetSelectedIndexes([]int{})
		}
		if a.eventView != nil {
			_ = a.eventView.SetSelectedIndexes([]int{})
		}
		a.selectionChanging = false
	}
	a.updateDetails()
}

func (a *app) onDeviceMonitorChanged(model.DeviceSnapshot, bool) {
	a.updateSummary()
	a.updateDetails()
}

func (a *app) shouldAutoSelectLatestEvent() bool {
	return !a.watchFocusActive && selectedIndex(a.deviceView) < 0 && selectedIndex(a.usbChangeView) < 0
}

func (a *app) shouldRefreshWatchDetailsForEvent(event model.Event) bool {
	if !a.hasWatchedDevice || !a.watchFocusActive {
		return false
	}
	if sameDeviceForHistory(a.watchedDevice, event.Device) {
		return true
	}
	return isSystemPowerEvent(event)
}

func (a *app) currentDeviceSelectionTarget() (model.DeviceSnapshot, bool) {
	if idx := selectedIndex(a.deviceView); idx >= 0 {
		if device, ok := a.devices.Item(idx); ok {
			return device, true
		}
	}
	if a.hasWatchedDevice && a.watchFocusActive && selectedIndex(a.eventView) < 0 && selectedIndex(a.usbChangeView) < 0 {
		return a.watchedDevice, true
	}
	return model.DeviceSnapshot{}, false
}

func (a *app) restoreDeviceSelection(target model.DeviceSnapshot) bool {
	idx := -1
	if a.devices != nil {
		idx = a.devices.IndexOfDevice(target)
	}
	if idx < 0 || a.deviceView == nil {
		return false
	}
	a.selectionChanging = true
	_ = a.deviceView.SetSelectedIndexes([]int{idx})
	a.selectionChanging = false
	if device, ok := a.devices.Item(idx); ok {
		a.watchedDevice = device
		a.hasWatchedDevice = true
		a.watchFocusActive = true
	}
	return true
}

func findDeviceIndex(devices []model.DeviceSnapshot, target model.DeviceSnapshot) int {
	for i, device := range devices {
		if sameDeviceForSelection(device, target) {
			return i
		}
	}
	return -1
}

func (a *app) openSelectedDeviceDetailsWindow() {
	if idx := selectedIndex(a.deviceView); idx >= 0 {
		if device, ok := a.devices.Item(idx); ok {
			a.openDeviceDetailsWindow(device)
		}
	}
}

func (a *app) openDeviceDetailsWindow(device model.DeviceSnapshot) {
	history := a.events.DeviceHistory(device, 200)
	sequence := newEventTableModel(200)
	sequence.SetLanguage(a.language)
	sequence.Set(history)

	var dlg *walk.Dialog
	var closeButton *walk.PushButton
	text := a.text()
	if _, err := (d.Dialog{
		AssignTo: &dlg,
		Title:    "USB device details - " + device.DisplayName(),
		Size:     d.Size{Width: 1080, Height: 760},
		MinSize:  d.Size{Width: 860, Height: 560},
		Icon:     a.icon,
		Layout:   d.VBox{Margins: d.Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
		Children: []d.Widget{
			d.Label{Text: device.DisplayName(), EllipsisMode: d.EllipsisEnd},
			d.HSplitter{
				Children: []d.Widget{
					d.Composite{
						Layout: d.VBox{MarginsZero: true},
						Children: []d.Widget{
							d.Label{Text: text.selectedSequenceTitle},
							d.TableView{
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								MaxSize:          d.Size{Height: 220},
								Columns: []d.TableViewColumn{
									{Title: text.eventColumnTitles[0], Width: 90},
									{Title: text.eventColumnTitles[1], Width: 145},
									{Title: text.eventColumnTitles[2], Width: 125},
									{Title: text.eventColumnTitles[3], Width: 85},
									{Title: text.eventColumnTitles[4], Width: 110},
									{Title: text.eventColumnTitles[5], Width: 210},
									{Title: text.eventColumnTitles[6], Width: 360},
								},
								Model: sequence,
							},
							d.Label{Text: text.diagnosticDetailsTitle},
							d.TextEdit{
								ReadOnly: true,
								VScroll:  true,
								HScroll:  true,
								Text:     formatDevice(device, a.language, a.devices.IsMonitored(device), history),
							},
						},
					},
					d.Composite{
						Layout: d.VBox{MarginsZero: true},
						Children: []d.Widget{
							d.Label{Text: text.rawDetailsTitle},
							d.TextEdit{
								ReadOnly: true,
								VScroll:  true,
								HScroll:  true,
								Text:     formatDeviceRaw(device, history),
							},
						},
					},
				},
			},
			d.Composite{
				Layout: d.HBox{MarginsZero: true},
				Children: []d.Widget{
					d.HSpacer{},
					d.PushButton{
						AssignTo: &closeButton,
						Text:     "Close",
						OnClicked: func() {
							if dlg != nil {
								dlg.Accept()
							}
						},
					},
				},
			},
		},
		CancelButton: &closeButton,
	}).Run(a.mw); err != nil {
		walk.MsgBox(a.mw, "USB device details", "failed to open device details: "+err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
	}
}

func (a *app) updateDetails() {
	if a.details == nil {
		return
	}
	if idx := selectedIndex(a.eventView); idx >= 0 {
		if event, ok := a.events.Item(idx); ok {
			history := a.events.DeviceHistory(event.Device, 100)
			a.setSelectedSequence(history)
			a.setDetails(
				formatEvent(event, a.language, a.devices.IsMonitored(event.Device), a.events.WakeCorrelation(event.Time, 30*time.Second)),
				formatEventRaw(event),
			)
			return
		}
	}
	if idx := selectedIndex(a.usbChangeView); idx >= 0 {
		if event, ok := a.usbChanges.Item(idx); ok {
			history := a.events.DeviceHistory(event.Device, 100)
			a.setSelectedSequence(history)
			a.setDetails(
				formatEvent(event, a.language, a.devices.IsMonitored(event.Device), a.events.WakeCorrelation(event.Time, 30*time.Second)),
				formatEventRaw(event),
			)
			return
		}
	}
	if idx := selectedIndex(a.deviceView); idx >= 0 {
		if device, ok := a.devices.Item(idx); ok {
			history := a.events.DeviceHistory(device, 100)
			a.setSelectedSequence(history)
			a.setDetails(
				formatDevice(device, a.language, a.devices.IsMonitored(device), history),
				formatDeviceRaw(device, history),
			)
			return
		}
	}
	if a.hasWatchedDevice && a.watchFocusActive {
		history := a.events.DeviceHistory(a.watchedDevice, 100)
		a.setSelectedSequence(history)
		a.setDetails(
			formatDevice(a.watchedDevice, a.language, a.devices.IsMonitored(a.watchedDevice), history),
			formatDeviceRaw(a.watchedDevice, history),
		)
		return
	}
	a.setSelectedSequence(nil)
	a.setDetails(a.text().emptyDetails, "{}")
}

func (a *app) setDetails(summary, raw string) {
	if a.details != nil {
		a.details.SetText(summary)
	}
	if a.rawDetails != nil {
		a.rawDetails.SetText(raw)
	}
}

func (a *app) setSelectedSequence(events []model.Event) {
	if a.sequence != nil {
		a.sequence.Set(events)
	}
}

func (a *app) withSessionRaw(event model.Event) model.Event {
	raw := make(map[string]string, len(event.Raw)+8)
	for key, value := range event.Raw {
		raw[key] = value
	}
	if event.Device.CorrelationID == "" {
		event.Device.CorrelationID = a.correlationID
	}
	raw["session_started_at"] = a.sessionStarted.Format(time.RFC3339Nano)
	raw["correlation_id"] = a.correlationID
	if hasDeviceIdentity(event.Device) {
		for key, value := range model.DeviceEvidenceRaw(event.Device) {
			if _, ok := raw[key]; !ok {
				raw[key] = value
			}
		}
		raw["diagnostic_summary"] = strings.Join(diagnosticSummary(event.Device, nil), " | ")
		raw["diagnostic_causes"] = formatDiagnosticCauses(model.DiagnosticCauses(event.Device, nil))
	}
	event.Raw = raw
	return event
}

func selectedIndex(tv *walk.TableView) int {
	if tv == nil {
		return -1
	}
	indexes := tv.SelectedIndexes()
	if len(indexes) == 0 {
		return -1
	}
	return indexes[0]
}

func (a *app) updateStatus(text string) {
	a.statusText = text
	if a.statusLabel == nil {
		return
	}
	strings := a.text()
	admin := strings.standardUser
	if platform.IsAdmin() {
		admin = strings.administrator
	}
	if a.etwHelperAdmin != nil {
		helper := strings.standardUser
		if *a.etwHelperAdmin {
			helper = strings.administrator
		}
		admin += " | ETW helper: " + helper
	}
	status := localizeStatus(text, a.language)
	if a.etwRunning {
		status = strings.preciseETWRequested + "; " + status
	}
	a.statusLabel.SetText(fmt.Sprintf("%s: %s | %s", strings.monitoringPrefix, status, versionOrDev(a.cfg.Version)))
	if a.privilegeLabel != nil {
		a.privilegeLabel.SetText(strings.privilegePrefix + ": " + admin)
	}
	a.updateSummary()
}

func (a *app) updateSummary() {
	if a.summaryLabel == nil {
		return
	}
	stats := a.events.Stats()
	devices := a.devices.All()
	lowPower := 0
	for _, device := range devices {
		if model.IsLowPowerState(device.PowerState) {
			lowPower++
		}
	}
	strings := a.text()
	a.summaryLabel.SetText(fmt.Sprintf(
		strings.summaryFormat,
		len(devices),
		a.devices.MonitoredCount(),
		lowPower,
		stats.SuspectedSuspend,
		stats.Resume,
		a.events.RowCount(),
	))
	if a.logPathLabel != nil && a.logger != nil {
		a.logPathLabel.SetText(strings.logPrefix + ": " + a.logger.Path())
	}
}

func (a *app) isEventMonitored(event model.Event) bool {
	if a.devices == nil {
		return true
	}
	if event.Source == model.SourceUSBPcap {
		return true
	}
	if hasDeviceIdentity(event.Device) && !eventMatchesTarget(event, a.currentTargetFilterIndex()) {
		return false
	}
	keys := deviceMonitorKeys(event.Device)
	if len(keys) == 0 {
		return true
	}
	for _, key := range keys {
		if !a.devices.IsMonitoredKey(key) {
			return false
		}
	}
	return true
}

func (a *app) applyEventFilters() {
	if a.events == nil {
		return
	}
	a.events.SetFilter(a.currentEventFilter())
	a.updateDetails()
	a.updateSummary()
}

func (a *app) applyTargetFilter() {
	a.refreshDevices()
	a.applyEventFilters()
	if a.usbChanges != nil {
		a.usbChanges.SetFilter(eventFilter{TargetIndex: a.currentTargetFilterIndex(), LevelIndex: 2})
	}
}

func (a *app) currentEventFilter() eventFilter {
	filter := eventFilter{}
	if a.eventTypeFilter != nil {
		filter.TypeIndex = a.eventTypeFilter.CurrentIndex()
	}
	filter.TargetIndex = a.currentTargetFilterIndex()
	if a.confidenceFilter != nil {
		filter.ConfidenceIndex = a.confidenceFilter.CurrentIndex()
	}
	if a.levelFilter != nil {
		filter.LevelIndex = a.levelFilter.CurrentIndex()
	}
	if a.eventSearch != nil {
		filter.Query = a.eventSearch.Text()
	}
	return filter
}

func (a *app) currentTargetFilterIndex() int {
	if a.targetFilter == nil {
		return 1
	}
	index := a.targetFilter.CurrentIndex()
	if index < 0 {
		return 1
	}
	return index
}

func (a *app) text() uiStrings {
	return stringsFor(a.language)
}

func (a *app) onLanguageChanged() {
	if a.localizing || a.languageCombo == nil {
		return
	}
	next := languageFromIndex(a.languageCombo.CurrentIndex())
	if next == a.language {
		return
	}
	a.language = next
	a.applyLanguage()
}

func (a *app) applyLanguage() {
	text := a.text()
	a.localizing = true
	defer func() { a.localizing = false }()

	setButtonText(a.refreshButton, text.refreshButton)
	setButtonText(a.startETWButton, text.startETWButton)
	setButtonText(a.stopETWButton, text.stopETWButton)
	setButtonText(a.startUSBPcapButton, text.startUSBPcapButton)
	setButtonText(a.stopUSBPcapButton, text.stopUSBPcapButton)
	setButtonText(a.openLogsButton, text.openLogsButton)
	setButtonText(a.exportButton, text.exportVisibleButton)
	setLabelText(a.languageLabel, text.languageLabel)
	setLabelText(a.devicesLabel, text.connectedDevicesTitle)
	setLabelText(a.groupsLabel, text.adapterGroupsTitle)
	setLabelText(a.usbChangesLabel, text.usbChangesTitle)
	setLabelText(a.timelineLabel, text.timelineTitle)
	setLabelText(a.sequenceLabel, text.selectedSequenceTitle)
	setLabelText(a.detailsLabel, text.diagnosticDetailsTitle)
	setLabelText(a.rawDetailsLabel, text.rawDetailsTitle)
	setLabelText(a.typeLabel, text.typeLabel)
	setLabelText(a.targetLabel, text.targetLabel)
	setLabelText(a.confidenceLabel, text.confidenceLabel)
	setLabelText(a.levelLabel, text.levelLabel)
	if a.statusGroup != nil {
		_ = a.statusGroup.SetTitle(text.monitoringStatusTitle)
	}
	setComboModel(a.languageCombo, text.languageOptions, a.language.index())
	setComboModel(a.eventTypeFilter, text.typeOptions, selectedComboIndex(a.eventTypeFilter))
	setComboModel(a.targetFilter, text.targetOptions, selectedComboIndex(a.targetFilter))
	setComboModel(a.confidenceFilter, text.confidenceOptions, selectedComboIndex(a.confidenceFilter))
	setComboModel(a.levelFilter, text.levelOptions, selectedComboIndex(a.levelFilter))
	if a.eventSearch != nil {
		_ = a.eventSearch.SetCueBanner(text.searchCue)
	}
	setColumnTitles(a.deviceView, text.deviceColumnTitles)
	setColumnTitles(a.groupView, text.groupColumnTitles)
	setColumnTitles(a.eventView, text.eventColumnTitles)
	setColumnTitles(a.usbChangeView, text.eventColumnTitles)
	setColumnTitles(a.sequenceView, text.eventColumnTitles)
	if a.devices != nil {
		a.devices.SetLanguage(a.language)
	}
	if a.events != nil {
		a.events.SetLanguage(a.language)
	}
	if a.usbChanges != nil {
		a.usbChanges.SetLanguage(a.language)
	}
	if a.sequence != nil {
		a.sequence.SetLanguage(a.language)
	}
	if a.showAction != nil {
		_ = a.showAction.SetText(text.trayShow)
	}
	if a.logAction != nil {
		_ = a.logAction.SetText(text.trayOpenLogs)
	}
	if a.exitAction != nil {
		_ = a.exitAction.SetText(text.trayExit)
	}
	if a.notifyIcon != nil {
		_ = a.notifyIcon.SetToolTip(text.trayTooltip)
	}
	status := a.statusText
	if status == "" {
		status = statusSimpleMonitorRunning
	}
	a.updateStatus(status)
	a.updateDetails()
}

func setButtonText(button *walk.PushButton, text string) {
	if button != nil {
		_ = button.SetText(text)
	}
}

func setLabelText(label *walk.Label, text string) {
	if label != nil {
		_ = label.SetText(text)
	}
}

func selectedComboIndex(combo *walk.ComboBox) int {
	if combo == nil {
		return 0
	}
	index := combo.CurrentIndex()
	if index < 0 {
		return 0
	}
	return index
}

func setComboModel(combo *walk.ComboBox, model []string, index int) {
	if combo == nil {
		return
	}
	_ = combo.SetModel(model)
	if index < 0 || index >= len(model) {
		index = 0
	}
	_ = combo.SetCurrentIndex(index)
}

func setColumnTitles(view *walk.TableView, titles []string) {
	if view == nil {
		return
	}
	columns := view.Columns()
	for i, title := range titles {
		if i >= columns.Len() {
			return
		}
		_ = columns.At(i).SetTitle(title)
	}
}

func (a *app) handleDeviceChange(wParam uintptr) {
	var typ model.EventType
	var msg string
	switch uint32(wParam) {
	case 0x8000:
		typ, msg = model.EventPnPArrival, "WM_DEVICECHANGE: device arrival"
	case 0x8004:
		typ, msg = model.EventPnPRemoval, "WM_DEVICECHANGE: device removal complete"
	case 0x0007:
		typ, msg = model.EventInfo, "WM_DEVICECHANGE: device nodes changed"
	default:
		typ, msg = model.EventInfo, fmt.Sprintf("WM_DEVICECHANGE: 0x%X", wParam)
	}
	a.addEvent(model.Event{
		Time:       time.Now(),
		Type:       typ,
		Source:     model.SourceDeviceChange,
		Confidence: model.ConfidenceMedium,
		Message:    msg,
		Raw:        map[string]string{"wparam": fmt.Sprintf("0x%X", wParam)},
	}, true)
	a.poller.RefreshNow()
}

func (a *app) handlePowerBroadcast(wParam uintptr) {
	const (
		pbtAPMSuspend           = 0x0004
		pbtAPMResumeSuspend     = 0x0007
		pbtAPMPowerStatusChange = 0x000A
		pbtAPMResumeAutomatic   = 0x0012
	)
	eventType := model.EventInfo
	confidence := model.ConfidenceMedium
	msg := fmt.Sprintf("WM_POWERBROADCAST: 0x%X", wParam)
	raw := map[string]string{"wparam": fmt.Sprintf("0x%X", wParam)}
	eventTime := time.Now()
	switch uint32(wParam) {
	case pbtAPMSuspend:
		eventType = model.EventSystemSleep
		confidence = model.ConfidenceHigh
		msg = "WM_POWERBROADCAST: system is suspending"
	case pbtAPMResumeSuspend:
		eventType = model.EventSystemWake
		confidence = model.ConfidenceHigh
		msg = "WM_POWERBROADCAST: system resumed after user-visible suspend"
	case pbtAPMResumeAutomatic:
		eventType = model.EventSystemWake
		confidence = model.ConfidenceHigh
		msg = "WM_POWERBROADCAST: system resumed automatically"
	case pbtAPMPowerStatusChange:
		msg = "WM_POWERBROADCAST: power status changed"
	}
	if eventType == model.EventSystemWake {
		correlation := a.events.WakeCorrelation(eventTime, 30*time.Second)
		for key, value := range powercfgLastWakeRaw() {
			raw[key] = value
		}
		for key, value := range wakeDiagnosticRaw(raw, correlation) {
			raw[key] = value
		}
		if formatted := formatEventSequence(correlation); formatted != "" {
			raw["wake_correlation"] = formatted
		}
	}
	a.addEvent(model.Event{
		Time:       eventTime,
		Type:       eventType,
		Source:     model.SourcePowerBroadcast,
		Confidence: confidence,
		Message:    msg,
		Raw:        raw,
	}, true)
	if eventType == model.EventSystemWake {
		a.addEvent(model.Event{
			Time:       eventTime,
			Type:       model.EventInfo,
			Source:     model.SourcePowercfgLastWake,
			Confidence: model.ConfidenceLow,
			Message:    "powercfg /lastwake captured",
			Raw:        raw,
		}, true)
		a.poller.RefreshNow()
	}
}

func powercfgLastWakeRaw() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "powercfg", "/lastwake").CombinedOutput()
	return powercfgLastWakeRawFromOutput(output, err)
}

func powercfgLastWakeRawFromOutput(output []byte, err error) map[string]string {
	raw := map[string]string{"lastwake_source": string(model.SourcePowercfgLastWake)}
	text := strings.TrimSpace(string(output))
	if text != "" {
		raw["lastwake"] = text
	}
	if err != nil {
		raw["lastwake_error"] = err.Error()
	}
	return raw
}

func wakeDiagnosticRaw(raw map[string]string, correlation []model.Event) map[string]string {
	out := make(map[string]string, 2)
	var reasons []string
	switch {
	case strings.TrimSpace(raw["lastwake"]) != "":
		out["wake_confidence"] = "high"
		reasons = append(reasons, "powercfg /lastwake returned wake source")
	case len(correlation) > 0:
		out["wake_confidence"] = "medium"
		reasons = append(reasons, "USB/PnP/D0/D3 event within 30 seconds of wake")
	case strings.TrimSpace(raw["lastwake_error"]) != "":
		out["wake_confidence"] = "unknown"
		reasons = append(reasons, "powercfg /lastwake unavailable: "+raw["lastwake_error"])
	default:
		out["wake_confidence"] = "low"
		reasons = append(reasons, "no wake source and no nearby USB change")
	}
	if len(reasons) > 0 {
		out["wake_reasons"] = strings.Join(reasons, " | ")
	}
	return out
}

func (a *app) startUSBPcap() {
	target, ok := a.currentDeviceSelectionTarget()
	if !ok || !hasDeviceIdentity(target) {
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventError,
			Source:     model.SourceUSBPcap,
			Confidence: model.ConfidenceLow,
			Message:    "select a connected USB device or parent hub before starting USBPcap capture",
		}, true)
		a.updateStatus(statusUSBPcapFailed)
		return
	}
	a.mu.Lock()
	if a.usbpcapStarting || a.usbpcapRunning {
		a.mu.Unlock()
		a.updateStatus(statusUSBPcapRunning)
		return
	}
	a.usbpcapStarting = true
	a.usbpcapStartID++
	startID := a.usbpcapStartID
	a.mu.Unlock()
	a.updateStatus(statusUSBPcapDiscovering)
	go a.startUSBPcapForDevice(startID, target)
}

func (a *app) startUSBPcapForDevice(startID int, target model.DeviceSnapshot) {
	fail := func(message string, raw map[string]string) {
		a.mw.Synchronize(func() {
			a.mu.Lock()
			if a.usbpcapStartID != startID {
				a.mu.Unlock()
				return
			}
			a.usbpcapStarting = false
			a.mu.Unlock()
			a.addEvent(model.Event{
				Time:       time.Now(),
				Type:       model.EventError,
				Source:     model.SourceUSBPcap,
				Confidence: model.ConfidenceLow,
				Device:     target,
				Message:    message,
				Raw:        raw,
			}, true)
			a.updateStatus(statusUSBPcapFailed)
		})
	}

	exePath, tried, err := usbpcap.DiscoverExecutable()
	if err != nil {
		fail("USBPcapCMD.exe not found; install USBPcap/Wireshark USBPcap or set USBPCAP_CMD", map[string]string{
			"usbpcap_error":       err.Error(),
			"usbpcap_paths_tried": strings.Join(tried, " | "),
		})
		return
	}
	interfaces, interfaceOutput, err := usbpcap.DiscoverInterfaces(exePath)
	if err != nil {
		fail("failed to list USBPcap interfaces: "+err.Error(), map[string]string{
			"usbpcap_cmd":        exePath,
			"usbpcap_interfaces": interfaceOutput,
		})
		return
	}
	outputPath := filepath.Join(a.logDir, a.logPrefix+"_usbpcap.pcapng")
	metadataPath := filepath.Join(a.logDir, a.logPrefix+"_usbpcap.json")
	plan, err := usbpcap.BuildPlan(exePath, interfaces, target, outputPath, metadataPath)
	if err != nil {
		fail("failed to build USBPcap capture plan: "+err.Error(), map[string]string{
			"usbpcap_cmd":        exePath,
			"usbpcap_interfaces": interfaceOutput,
		})
		return
	}
	if err := writeUSBPcapMetadata(plan, target, a.sessionStarted, a.correlationID); err != nil {
		fail("failed to write USBPcap metadata: "+err.Error(), usbpcapRaw(plan, target, err.Error()))
		return
	}
	if !a.usbpcapStartStillActive(startID) {
		return
	}
	cmd := exec.Command(plan.ExePath, plan.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		fail("failed to start USBPcapCMD.exe: "+err.Error(), usbpcapRaw(plan, target, err.Error()))
		return
	}

	a.mw.Synchronize(func() {
		a.mu.Lock()
		if a.usbpcapStartID != startID || !a.usbpcapStarting {
			a.mu.Unlock()
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			return
		}
		a.usbpcapStarting = false
		a.usbpcapRunning = true
		a.usbpcapCmd = cmd
		a.usbpcapOutputPath = outputPath
		a.mu.Unlock()
		message := "USBPcap capture started for " + target.DisplayName()
		if plan.Warning != "" {
			message += "; " + plan.Warning
		}
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventInfo,
			Source:     model.SourceUSBPcap,
			Confidence: model.ConfidenceMedium,
			Device:     target,
			Message:    message,
			Raw:        usbpcapRaw(plan, target, ""),
		}, true)
		a.updateStatus(statusUSBPcapRunning)
	})
	go a.waitUSBPcap(cmd, target, outputPath, metadataPath)
}

func (a *app) usbpcapStartStillActive(startID int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.usbpcapStartID == startID && a.usbpcapStarting
}

func (a *app) waitUSBPcap(cmd *exec.Cmd, target model.DeviceSnapshot, outputPath, metadataPath string) {
	err := cmd.Wait()
	a.mw.Synchronize(func() {
		a.mu.Lock()
		if a.usbpcapCmd == cmd {
			a.usbpcapRunning = false
			a.usbpcapStarting = false
			a.usbpcapCmd = nil
			a.usbpcapOutputPath = ""
		}
		a.mu.Unlock()
		raw := map[string]string{
			"usbpcap_output":   outputPath,
			"usbpcap_metadata": metadataPath,
		}
		message := "USBPcap capture stopped"
		eventType := model.EventInfo
		confidence := model.ConfidenceMedium
		if err != nil {
			raw["usbpcap_exit"] = err.Error()
			message = "USBPcap capture stopped: " + err.Error()
			if !strings.Contains(strings.ToLower(err.Error()), "killed") {
				eventType = model.EventError
				confidence = model.ConfidenceLow
			}
		}
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       eventType,
			Source:     model.SourceUSBPcap,
			Confidence: confidence,
			Device:     target,
			Message:    message,
			Raw:        raw,
		}, true)
		if eventType == model.EventError {
			a.updateStatus(statusUSBPcapFailed)
			return
		}
		a.updateStatus(statusUSBPcapStopped)
	})
}

func (a *app) stopUSBPcap() {
	a.stopUSBPcapProcess(true)
}

func (a *app) stopUSBPcapProcess(addLogEvent bool) {
	a.mu.Lock()
	cmd := a.usbpcapCmd
	outputPath := a.usbpcapOutputPath
	a.usbpcapStartID++
	a.usbpcapStarting = false
	a.usbpcapRunning = false
	a.usbpcapCmd = nil
	a.usbpcapOutputPath = ""
	a.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if addLogEvent {
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventInfo,
			Source:     model.SourceUSBPcap,
			Confidence: model.ConfidenceMedium,
			Message:    "USBPcap stop requested",
			Raw: map[string]string{
				"usbpcap_output": outputPath,
			},
		}, true)
		a.updateStatus(statusUSBPcapStopped)
	}
}

func writeUSBPcapMetadata(plan usbpcap.Plan, target model.DeviceSnapshot, sessionStarted time.Time, correlationID string) error {
	data, err := json.MarshalIndent(map[string]interface{}{
		"session_started_at": sessionStarted.Format(time.RFC3339Nano),
		"correlation_id":     correlationID,
		"target":             target,
		"usbpcap_hints":      target.USBPcapHints,
		"capture_plan":       plan,
		"warning":            "USBPcap is software URB capture. Root-hub fallback can include sibling device traffic.",
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(plan.MetadataPath, data, 0644)
}

func usbpcapRaw(plan usbpcap.Plan, target model.DeviceSnapshot, errText string) map[string]string {
	raw := map[string]string{
		"usbpcap_cmd":               plan.ExePath,
		"usbpcap_args":              strings.Join(plan.Args, " "),
		"usbpcap_interface":         plan.Interface.Value,
		"usbpcap_interface_display": plan.Interface.Display,
		"usbpcap_output":            plan.OutputPath,
		"usbpcap_metadata":          plan.MetadataPath,
		"usbpcap_capture_all":       fmt.Sprint(plan.CaptureAll),
		"usbpcap_target":            target.DisplayName(),
		"usbpcap_discovery":         plan.DiscoverySummary,
		"usbpcap_hint_root_hub":     target.USBPcapHints.RootHub,
		"usbpcap_hint_bus_number":   target.USBPcapHints.BusNumber,
		"usbpcap_hint_device_addr":  target.USBPcapHints.DeviceAddress,
		"usbpcap_hint_bulk_in":      target.USBPcapHints.BulkInEndpoint,
		"usbpcap_hint_bulk_out":     target.USBPcapHints.BulkOutEndpoint,
		"usbpcap_endpoint_level":    target.USBPcapHints.EndpointConfidence,
	}
	if len(plan.DeviceAddresses) > 0 {
		parts := make([]string, 0, len(plan.DeviceAddresses))
		for _, address := range plan.DeviceAddresses {
			parts = append(parts, fmt.Sprint(address))
		}
		raw["usbpcap_device_addresses"] = strings.Join(parts, ",")
	}
	if len(plan.MatchReasons) > 0 {
		raw["usbpcap_match_reasons"] = strings.Join(plan.MatchReasons, " | ")
	}
	if plan.Warning != "" {
		raw["usbpcap_warning"] = plan.Warning
	}
	if errText != "" {
		raw["usbpcap_error"] = errText
	}
	return raw
}

func (a *app) startETW() {
	a.mu.Lock()
	if a.etwRunning {
		a.mu.Unlock()
		a.updateStatus(statusETWAlreadyRequested)
		return
	}
	stopFile := filepath.Join(a.logDir, "usb-suspend-watch-etw.stop")
	_ = os.Remove(stopFile)
	helperLog := logs.PathForPrefix(a.logDir, "usb-suspend-watch-etw", time.Now())
	etlPath := filepath.Join(a.logDir, a.logPrefix+"_etw.etl")
	var offset int64
	if info, err := os.Stat(helperLog); err == nil {
		offset = info.Size()
	}
	args := []string{
		"--etw-helper",
		"--log-dir", a.logDir,
		"--session", "UsbSuspendWatch-ETW",
		"--stop-file", stopFile,
		"--parent-pid", fmt.Sprint(os.Getpid()),
		"--etl-path", etlPath,
	}
	elevated := !platform.IsAdmin()
	err := platform.StartProcess(args, elevated)
	if err != nil {
		a.mu.Unlock()
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventError,
			Source:     model.SourceApp,
			Confidence: model.ConfidenceLow,
			Message:    "failed to start ETW helper: " + err.Error(),
		}, true)
		return
	}
	a.etwRunning = true
	a.etwStopFile = stopFile
	a.etwHelperAdmin = nil
	tailCtx, cancel := context.WithCancel(a.ctx)
	a.tailCancel = cancel
	a.mu.Unlock()

	go a.tailETWLog(tailCtx, helperLog, offset)
	a.addEvent(model.Event{
		Time:       time.Now(),
		Type:       model.EventInfo,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceHigh,
		Message:    "ETW helper start requested; UAC may appear if administrative rights are needed",
		Raw: map[string]string{
			"correlation_id": a.correlationID,
			"etl_path":       etlPath,
			"helper_log":     helperLog,
		},
	}, true)
	a.updateStatus(statusETWWaiting)
}

func (a *app) stopETW() {
	a.mu.Lock()
	stopFile := a.etwStopFile
	cancel := a.tailCancel
	a.etwRunning = false
	a.etwStopFile = ""
	a.tailCancel = nil
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if stopFile != "" {
		_ = os.WriteFile(stopFile, []byte(time.Now().Format(time.RFC3339)), 0644)
	}
	if a.statusLabel != nil {
		a.updateStatus(statusSimpleMonitorRunning)
	}
}

func (a *app) tailETWLog(ctx context.Context, path string, offset int64) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	startedAt := time.Now()
	startupSeen := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			events, next, err := logs.ReadEventsSince(path, offset)
			offset = next
			if err != nil {
				continue
			}
			if len(events) == 0 {
				if etwStartupHasTimedOut(startupSeen, startedAt, time.Now()) {
					a.handleETWStartupTimeout()
					return
				}
				continue
			}
			terminal := false
			terminalError := false
			a.mw.Synchronize(func() {
				for _, event := range events {
					if isETWHelperStartupEvent(event) {
						startupSeen = true
						a.captureETWHelperPrivilege(event)
					}
					if isTerminalETWHelperEvent(event) {
						terminal = true
						if event.Type == model.EventError {
							terminalError = true
						}
					}
					if !a.acceptTailedETWEvent(event) {
						continue
					}
					a.addEvent(event, false)
				}
				if terminal {
					a.clearETWRunning()
					if terminalError {
						a.updateStatus(statusETWFailed)
						return
					}
					a.updateStatus(statusETWStopped)
					return
				}
				a.updateStatus(statusETWReceived)
			})
			if terminal {
				return
			}
		}
	}
}

const etwStartupTimeout = 45 * time.Second

func etwStartupHasTimedOut(startupSeen bool, startedAt, now time.Time) bool {
	return !startupSeen && !startedAt.IsZero() && now.Sub(startedAt) >= etwStartupTimeout
}

func (a *app) handleETWStartupTimeout() {
	a.mw.Synchronize(func() {
		a.mu.Lock()
		stopFile := a.etwStopFile
		a.etwRunning = false
		a.etwStopFile = ""
		a.tailCancel = nil
		a.mu.Unlock()
		if stopFile != "" {
			_ = os.WriteFile(stopFile, []byte(time.Now().Format(time.RFC3339)), 0644)
		}
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventError,
			Source:     model.SourceApp,
			Confidence: model.ConfidenceLow,
			Message:    "ETW helper did not write a startup log within 45 seconds; UAC may have been cancelled, hidden, or blocked by policy",
		}, true)
		a.updateStatus(statusETWFailed)
	})
}

func (a *app) clearETWRunning() {
	a.mu.Lock()
	a.etwRunning = false
	a.etwStopFile = ""
	a.tailCancel = nil
	a.mu.Unlock()
}

func (a *app) captureETWHelperPrivilege(event model.Event) {
	value, ok := event.Raw["helper_admin"]
	if !ok {
		return
	}
	admin := strings.EqualFold(value, "true") || value == "1"
	a.etwHelperAdmin = &admin
}

func isETWHelperStartupEvent(event model.Event) bool {
	if event.Source != model.SourceApp {
		return false
	}
	if event.Type == model.EventError {
		return true
	}
	switch {
	case strings.HasPrefix(event.Message, "ETW helper starting"),
		strings.HasPrefix(event.Message, "ETW provider enabled:"),
		strings.HasPrefix(event.Message, "ETW provider unavailable:"),
		strings.HasPrefix(event.Message, "ETW helper running"):
		return true
	default:
		return false
	}
}

func isTerminalETWHelperEvent(event model.Event) bool {
	if event.Source != model.SourceApp {
		return false
	}
	if event.Type == model.EventError {
		return true
	}
	if event.Type != model.EventInfo {
		return false
	}
	switch event.Message {
	case "ETW helper stop file detected", "ETW helper parent process exited":
		return true
	default:
		return false
	}
}

func (a *app) acceptTailedETWEvent(event model.Event) bool {
	if a.levelFilter == nil {
		return eventMatchesDisplayLevel(event, 0)
	}
	return eventMatchesDisplayLevel(event, a.levelFilter.CurrentIndex())
}

func (a *app) openLogFolder() {
	_ = exec.Command("explorer.exe", a.logDir).Start()
}

func (a *app) exportVisibleLog() {
	text := a.text()
	dlg := new(walk.FileDialog)
	dlg.Title = text.exportDialogTitle
	dlg.FilePath = filepath.Join(a.logDir, "usb-suspend-watch-export.jsonl")
	dlg.Filter = "JSON Lines (*.jsonl)|*.jsonl|All files (*.*)|*.*"
	ok, err := dlg.ShowSave(a.mw)
	if err != nil || !ok {
		return
	}
	if err := logs.ExportEvents(dlg.FilePath, a.events.All()); err != nil {
		walk.MsgBox(a.mw, text.exportFailedTitle, err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(a.mw, text.exportCompleteTitle, dlg.FilePath, walk.MsgBoxIconInformation)
}

func (a *app) hideToTray() {
	if a.mw == nil {
		return
	}
	a.mw.SetVisible(false)
	if a.notifyIcon != nil {
		_ = a.notifyIcon.SetToolTip(a.text().trayTooltip)
	}
}

func (a *app) showFromTray() {
	if a.mw == nil {
		return
	}
	a.mw.SetVisible(true)
	win.ShowWindow(a.mw.Handle(), win.SW_RESTORE)
	win.BringWindowToTop(a.mw.Handle())
}

func (a *app) notifyImportantEvent(event model.Event) {
	if a.notifyIcon == nil || !isTrayNotificationEvent(event.Type) {
		return
	}
	if a.mw != nil && a.mw.Visible() && !win.IsIconic(a.mw.Handle()) {
		return
	}
	text := a.text()
	device := event.Device.DisplayName()
	if strings.TrimSpace(device) == "" || device == "(unknown USB device)" {
		device = text.unknownUSBDevice
	}
	message := device
	if event.Message != "" {
		message = event.Message
	}
	switch event.Type {
	case model.EventError:
		_ = a.notifyIcon.ShowError("USB Suspend Watch", message)
	case model.EventResume, model.EventPowerD0Entry:
		_ = a.notifyIcon.ShowInfo(text.notifyResumeTitle, message)
	default:
		_ = a.notifyIcon.ShowWarning(text.notifySuspendTitle, message)
	}
}

func isTrayNotificationEvent(eventType model.EventType) bool {
	switch eventType {
	case model.EventSuspectSuspend, model.EventResume, model.EventError:
		return true
	default:
		return false
	}
}
