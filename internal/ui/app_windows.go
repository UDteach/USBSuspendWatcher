package ui

import (
	"context"
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
	"usb-suspend-watch/internal/platform"
)

type Config struct {
	Version string
}

type app struct {
	cfg Config

	mw               *walk.MainWindow
	refreshButton    *walk.PushButton
	startETWButton   *walk.PushButton
	stopETWButton    *walk.PushButton
	openLogsButton   *walk.PushButton
	exportButton     *walk.PushButton
	languageLabel    *walk.Label
	languageCombo    *walk.ComboBox
	statusGroup      *walk.GroupBox
	deviceView       *walk.TableView
	eventView        *walk.TableView
	details          *walk.TextEdit
	devicesLabel     *walk.Label
	timelineLabel    *walk.Label
	typeLabel        *walk.Label
	confidenceLabel  *walk.Label
	levelLabel       *walk.Label
	statusLabel      *walk.Label
	summaryLabel     *walk.Label
	privilegeLabel   *walk.Label
	logPathLabel     *walk.Label
	eventTypeFilter  *walk.ComboBox
	confidenceFilter *walk.ComboBox
	levelFilter      *walk.ComboBox
	eventSearch      *walk.LineEdit
	notifyIcon       *walk.NotifyIcon
	showAction       *walk.Action
	logAction        *walk.Action
	exitAction       *walk.Action
	icon             *walk.Icon

	devices *deviceTableModel
	events  *eventTableModel
	poller  *monitor.Poller
	logger  *logs.EventLogger
	logDir  string

	ctx        context.Context
	cancel     context.CancelFunc
	tailCancel context.CancelFunc

	wndProc    uintptr
	oldWndProc uintptr

	etwRunning  bool
	etwStopFile string
	mu          sync.Mutex

	selectionChanging bool
	localizing        bool
	language          displayLanguage
	statusText        string
}

func Run(cfg Config) error {
	logDir, err := logs.ResolveLogDir()
	if err != nil {
		return err
	}
	logger, err := logs.NewEventLogger(logDir)
	if err != nil {
		return err
	}
	defer logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	a := &app{
		cfg:      cfg,
		devices:  newDeviceTableModel(),
		events:   newEventTableModel(2000),
		poller:   monitor.NewPoller(2 * time.Second),
		logger:   logger,
		logDir:   logDir,
		ctx:      ctx,
		cancel:   cancel,
		language: languageJapanese,
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
	a.refreshDevices()
	a.addEvent(model.Event{
		Time:       time.Now(),
		Type:       model.EventInfo,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceHigh,
		Message:    "USB Suspend Watch started",
	}, true)

	a.updateStatus(statusSimpleMonitorRunning)
	a.mw.Run()
	return nil
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
									{Title: text.deviceColumnTitles[2], Width: 110},
									{Title: text.deviceColumnTitles[3], Width: 70},
									{Title: text.deviceColumnTitles[4], Width: 90},
									{Title: text.deviceColumnTitles[5], Width: 160},
									{Title: text.deviceColumnTitles[6], Width: 115},
								},
								Model: a.devices,
								OnSelectedIndexesChanged: func() {
									a.onDeviceSelectionChanged()
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
							d.TextEdit{
								AssignTo: &a.details,
								ReadOnly: true,
								VScroll:  true,
								HScroll:  true,
								Text:     text.emptyDetails,
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
		if a.oldWndProc == 0 {
			return win.DefWindowProc(hwnd, msg, wParam, lParam)
		}
		return win.CallWindowProc(a.oldWndProc, hwnd, msg, wParam, lParam)
	})
	a.oldWndProc = win.SetWindowLongPtr(a.mw.Handle(), win.GWLP_WNDPROC, a.wndProc)
}

func (a *app) cleanup() {
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
	devices := a.poller.Snapshot()
	sort.Slice(devices, func(i, j int) bool {
		return strings.ToLower(devices[i].DisplayName()) < strings.ToLower(devices[j].DisplayName())
	})
	a.devices.Set(devices)
	a.updateDetails()
	a.updateStatus(statusSimpleMonitorRunning)
}

func (a *app) addEvent(event model.Event, persist bool) {
	if !a.isEventMonitored(event) {
		return
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	a.events.Add(event)
	if persist {
		_ = a.logger.Append(event)
	}
	if a.eventView != nil && a.events.RowCount() > 0 && eventMatchesFilter(event, a.events.filter) {
		_ = a.eventView.SetSelectedIndexes([]int{a.events.RowCount() - 1})
	}
	a.updateSummary()
	a.notifyImportantEvent(event)
}

func (a *app) onDeviceSelectionChanged() {
	if a.selectionChanging {
		return
	}
	if selectedIndex(a.deviceView) >= 0 && a.eventView != nil {
		a.selectionChanging = true
		_ = a.eventView.SetSelectedIndexes([]int{})
		a.selectionChanging = false
	}
	a.updateDetails()
}

func (a *app) onEventSelectionChanged() {
	if a.selectionChanging {
		return
	}
	if selectedIndex(a.eventView) >= 0 && a.deviceView != nil {
		a.selectionChanging = true
		_ = a.deviceView.SetSelectedIndexes([]int{})
		a.selectionChanging = false
	}
	a.updateDetails()
}

func (a *app) onDeviceMonitorChanged(model.DeviceSnapshot, bool) {
	a.updateSummary()
	a.updateDetails()
}

func (a *app) updateDetails() {
	if a.details == nil {
		return
	}
	if idx := selectedIndex(a.eventView); idx >= 0 {
		if event, ok := a.events.Item(idx); ok {
			a.details.SetText(formatEvent(event, a.language, a.devices.IsMonitored(event.Device)))
			return
		}
	}
	if idx := selectedIndex(a.deviceView); idx >= 0 {
		if device, ok := a.devices.Item(idx); ok {
			a.details.SetText(formatDevice(device, a.language, a.devices.IsMonitored(device)))
			return
		}
	}
	a.details.SetText(a.text().emptyDetails)
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

func (a *app) currentEventFilter() eventFilter {
	filter := eventFilter{}
	if a.eventTypeFilter != nil {
		filter.TypeIndex = a.eventTypeFilter.CurrentIndex()
	}
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
	setButtonText(a.openLogsButton, text.openLogsButton)
	setButtonText(a.exportButton, text.exportVisibleButton)
	setLabelText(a.languageLabel, text.languageLabel)
	setLabelText(a.devicesLabel, text.connectedDevicesTitle)
	setLabelText(a.timelineLabel, text.timelineTitle)
	setLabelText(a.typeLabel, text.typeLabel)
	setLabelText(a.confidenceLabel, text.confidenceLabel)
	setLabelText(a.levelLabel, text.levelLabel)
	if a.statusGroup != nil {
		_ = a.statusGroup.SetTitle(text.monitoringStatusTitle)
	}
	setComboModel(a.languageCombo, text.languageOptions, a.language.index())
	setComboModel(a.eventTypeFilter, text.typeOptions, selectedComboIndex(a.eventTypeFilter))
	setComboModel(a.confidenceFilter, text.confidenceOptions, selectedComboIndex(a.confidenceFilter))
	setComboModel(a.levelFilter, text.levelOptions, selectedComboIndex(a.levelFilter))
	if a.eventSearch != nil {
		_ = a.eventSearch.SetCueBanner(text.searchCue)
	}
	setColumnTitles(a.deviceView, text.deviceColumnTitles)
	setColumnTitles(a.eventView, text.eventColumnTitles)
	if a.devices != nil {
		a.devices.SetLanguage(a.language)
	}
	if a.events != nil {
		a.events.SetLanguage(a.language)
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
	}, true)
	a.poller.RefreshNow()
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
				continue
			}
			terminal := false
			terminalError := false
			a.mw.Synchronize(func() {
				for _, event := range events {
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

func (a *app) clearETWRunning() {
	a.mu.Lock()
	a.etwRunning = false
	a.etwStopFile = ""
	a.tailCancel = nil
	a.mu.Unlock()
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
