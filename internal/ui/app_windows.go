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
	deviceView       *walk.TableView
	eventView        *walk.TableView
	details          *walk.TextEdit
	statusLabel      *walk.Label
	summaryLabel     *walk.Label
	privilegeLabel   *walk.Label
	logPathLabel     *walk.Label
	eventTypeFilter  *walk.ComboBox
	confidenceFilter *walk.ComboBox
	eventSearch      *walk.LineEdit
	notifyIcon       *walk.NotifyIcon
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
		cfg:     cfg,
		devices: newDeviceTableModel(),
		events:  newEventTableModel(2000),
		poller:  monitor.NewPoller(2 * time.Second),
		logger:  logger,
		logDir:  logDir,
		ctx:     ctx,
		cancel:  cancel,
	}

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

	a.updateStatus("simple monitor running")
	a.mw.Run()
	return nil
}

func (a *app) createWindow() error {
	var err error
	a.icon, err = buildIcon()
	if err != nil {
		return err
	}

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
					d.PushButton{Text: "更新 / Refresh", OnClicked: a.refreshDevices},
					d.PushButton{Text: "ETW開始 実験 / Start ETW experimental", OnClicked: a.startETW},
					d.PushButton{Text: "ETW停止 / Stop ETW", OnClicked: a.stopETW},
					d.PushButton{Text: "ログフォルダ / Open logs", OnClicked: a.openLogFolder},
					d.PushButton{Text: "表示ログ出力 / Export visible", OnClicked: a.exportVisibleLog},
				},
			},
			d.GroupBox{
				Title:  "監視状態 / Monitoring status",
				Layout: d.Grid{Columns: 3, Margins: d.Margins{Left: 8, Top: 6, Right: 8, Bottom: 6}, Spacing: 6},
				Children: []d.Widget{
					d.Label{AssignTo: &a.statusLabel, Text: "監視 / Monitoring: starting", ColumnSpan: 2, EllipsisMode: d.EllipsisEnd},
					d.Label{AssignTo: &a.privilegeLabel, Text: "権限 / Privilege: checking", EllipsisMode: d.EllipsisEnd},
					d.Label{AssignTo: &a.summaryLabel, Text: "USB: 0 | Low power: 0 | Suspected suspend: 0 | Resume: 0", ColumnSpan: 2, EllipsisMode: d.EllipsisEnd},
					d.Label{AssignTo: &a.logPathLabel, Text: "ログ / Log: " + a.logDir, EllipsisMode: d.EllipsisPath},
				},
			},
			d.HSplitter{
				Children: []d.Widget{
					d.Composite{
						Layout: d.VBox{MarginsZero: true},
						Children: []d.Widget{
							d.Label{Text: "接続中USB / Connected USB devices"},
							d.TableView{
								AssignTo:         &a.deviceView,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								Columns: []d.TableViewColumn{
									{Title: "Name", Width: 250},
									{Title: "VID/PID", Width: 110},
									{Title: "Power", Width: 70},
									{Title: "Enumerator", Width: 90},
									{Title: "Location", Width: 180},
									{Title: "Last seen", Width: 130},
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
									d.Label{Text: "Suspend / Resume タイムライン / Timeline"},
									d.Composite{
										Layout: d.VBox{MarginsZero: true, Spacing: 3},
										Children: []d.Widget{
											d.Composite{
												Layout: d.HBox{MarginsZero: true},
												Children: []d.Widget{
													d.Label{Text: "種別 / Type"},
													d.ComboBox{
														AssignTo:              &a.eventTypeFilter,
														Model:                 []string{"すべて / All", "Suspend疑い / Suspected", "Resume", "PnP", "Error"},
														CurrentIndex:          0,
														StretchFactor:         1,
														OnCurrentIndexChanged: a.applyEventFilters,
													},
													d.Label{Text: "信頼度 / Confidence"},
													d.ComboBox{
														AssignTo:              &a.confidenceFilter,
														Model:                 []string{"すべて / All", "High+Medium", "High only"},
														CurrentIndex:          0,
														StretchFactor:         1,
														OnCurrentIndexChanged: a.applyEventFilters,
													},
												},
											},
											d.LineEdit{
												AssignTo:      &a.eventSearch,
												CueBanner:     "検索 / Search device, VID/PID, Instance ID, message",
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
											{Title: "Mark", Width: 95},
											{Title: "Time", Width: 150},
											{Title: "Event", Width: 125},
											{Title: "Confidence", Width: 90},
											{Title: "Source", Width: 120},
											{Title: "Device", Width: 240},
											{Title: "Message", Width: 340},
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
								Text:     "デバイスまたはイベントを選択してください。\r\nSelect a device or event to inspect details.",
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
	return nil
}

func (a *app) setupNotifyIcon() {
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
	_ = showAction.SetText("表示 / Show")
	showAction.Triggered().Attach(a.showFromTray)
	logAction := walk.NewAction()
	_ = logAction.SetText("ログフォルダ / Open logs")
	logAction.Triggered().Attach(a.openLogFolder)
	exitAction := walk.NewAction()
	_ = exitAction.SetText("終了 / Exit")
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
	a.updateStatus("simple monitor running")
}

func (a *app) addEvent(event model.Event, persist bool) {
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

func (a *app) updateDetails() {
	if a.details == nil {
		return
	}
	if idx := selectedIndex(a.eventView); idx >= 0 {
		if event, ok := a.events.Item(idx); ok {
			a.details.SetText(formatEvent(event))
			return
		}
	}
	if idx := selectedIndex(a.deviceView); idx >= 0 {
		if device, ok := a.devices.Item(idx); ok {
			a.details.SetText(formatDevice(device))
			return
		}
	}
	a.details.SetText("デバイスまたはイベントを選択してください。\r\nSelect a device or event to inspect details.")
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
	if a.statusLabel == nil {
		return
	}
	admin := "standard user"
	if platform.IsAdmin() {
		admin = "administrator"
	}
	if a.etwRunning {
		text = "precise ETW requested; " + text
	}
	a.statusLabel.SetText(fmt.Sprintf("監視 / Monitoring: %s | v%s", text, versionOrDev(a.cfg.Version)))
	if a.privilegeLabel != nil {
		a.privilegeLabel.SetText("権限 / Privilege: " + admin)
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
	a.summaryLabel.SetText(fmt.Sprintf(
		"USB: %d | 低電力 / Low power: %d | Suspend疑い / Suspected: %d | Resume: %d | 表示 / Visible: %d",
		len(devices),
		lowPower,
		stats.SuspectedSuspend,
		stats.Resume,
		a.events.RowCount(),
	))
	if a.logPathLabel != nil && a.logger != nil {
		a.logPathLabel.SetText("ログ / Log: " + a.logger.Path())
	}
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
	if a.eventSearch != nil {
		filter.Query = a.eventSearch.Text()
	}
	return filter
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
	if os.Getenv("USB_SUSPEND_WATCH_EXPERIMENTAL_ETW") != "1" {
		a.addEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventInfo,
			Source:     model.SourceApp,
			Confidence: model.ConfidenceHigh,
			Message:    "ETW helper is disabled in the release UI; set USB_SUSPEND_WATCH_EXPERIMENTAL_ETW=1 only for lab testing",
		}, true)
		a.updateStatus("ETW helper disabled in release UI")
		return
	}

	a.mu.Lock()
	if a.etwRunning {
		a.mu.Unlock()
		a.updateStatus("ETW helper is already requested")
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
	a.updateStatus("waiting for ETW helper events")
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
		a.updateStatus("simple monitor running")
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
			a.mw.Synchronize(func() {
				for _, event := range events {
					a.addEvent(event, false)
				}
				a.updateStatus("ETW helper events received")
			})
		}
	}
}

func (a *app) openLogFolder() {
	_ = exec.Command("explorer.exe", a.logDir).Start()
}

func (a *app) exportVisibleLog() {
	dlg := new(walk.FileDialog)
	dlg.Title = "表示ログ出力 / Export visible JSONL log"
	dlg.FilePath = filepath.Join(a.logDir, "usb-suspend-watch-export.jsonl")
	dlg.Filter = "JSON Lines (*.jsonl)|*.jsonl|All files (*.*)|*.*"
	ok, err := dlg.ShowSave(a.mw)
	if err != nil || !ok {
		return
	}
	if err := logs.ExportEvents(dlg.FilePath, a.events.All()); err != nil {
		walk.MsgBox(a.mw, "Export failed", err.Error(), walk.MsgBoxIconError)
		return
	}
	walk.MsgBox(a.mw, "Export complete", dlg.FilePath, walk.MsgBoxIconInformation)
}

func (a *app) hideToTray() {
	if a.mw == nil {
		return
	}
	a.mw.SetVisible(false)
	if a.notifyIcon != nil {
		_ = a.notifyIcon.SetToolTip("USB Suspend Watch - monitoring")
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
	device := event.Device.DisplayName()
	if strings.TrimSpace(device) == "" || device == "(unknown USB device)" {
		device = "USB device"
	}
	message := device
	if event.Message != "" {
		message = event.Message
	}
	switch event.Type {
	case model.EventError:
		_ = a.notifyIcon.ShowError("USB Suspend Watch", message)
	case model.EventResume, model.EventPowerD0Entry:
		_ = a.notifyIcon.ShowInfo("USB Resume", message)
	default:
		_ = a.notifyIcon.ShowWarning("USB Suspend suspected", message)
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
