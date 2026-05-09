package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
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

	mw          *walk.MainWindow
	deviceView  *walk.TableView
	eventView   *walk.TableView
	details     *walk.TextEdit
	statusLabel *walk.Label
	notifyIcon  *walk.NotifyIcon
	icon        *walk.Icon

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

	go a.poller.Run(ctx)
	go a.consumePollerEvents()

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
					d.PushButton{Text: "Refresh", OnClicked: a.refreshDevices},
					d.PushButton{Text: "Start ETW (experimental)", OnClicked: a.startETW},
					d.PushButton{Text: "Stop ETW", OnClicked: a.stopETW},
					d.PushButton{Text: "Open log folder", OnClicked: a.openLogFolder},
					d.PushButton{Text: "Export visible log", OnClicked: a.exportVisibleLog},
				},
			},
			d.Label{AssignTo: &a.statusLabel, Text: "starting", EllipsisMode: d.EllipsisPath},
			d.HSplitter{
				Children: []d.Widget{
					d.Composite{
						Layout: d.VBox{MarginsZero: true},
						Children: []d.Widget{
							d.Label{Text: "Connected USB devices"},
							d.TableView{
								AssignTo:         &a.deviceView,
								AlternatingRowBG: true,
								ColumnsOrderable: true,
								Columns: []d.TableViewColumn{
									{Title: "Name", Width: 260},
									{Title: "VID/PID", Width: 110},
									{Title: "Power", Width: 70},
									{Title: "Enumerator", Width: 90},
									{Title: "Location", Width: 220},
								},
								Model: a.devices,
								OnSelectedIndexesChanged: func() {
									a.updateDetails()
								},
							},
						},
					},
					d.VSplitter{
						Children: []d.Widget{
							d.Composite{
								Layout: d.VBox{MarginsZero: true},
								Children: []d.Widget{
									d.Label{Text: "Suspend / Resume timeline"},
									d.TableView{
										AssignTo:         &a.eventView,
										AlternatingRowBG: true,
										ColumnsOrderable: true,
										Columns: []d.TableViewColumn{
											{Title: "Time", Width: 150},
											{Title: "Event", Width: 130},
											{Title: "Confidence", Width: 90},
											{Title: "Source", Width: 120},
											{Title: "Device", Width: 260},
											{Title: "Message", Width: 360},
										},
										Model: a.events,
										OnSelectedIndexesChanged: func() {
											a.updateDetails()
										},
									},
								},
							},
							d.TextEdit{
								AssignTo: &a.details,
								ReadOnly: true,
								VScroll:  true,
								HScroll:  true,
								Text:     "Select a device or event to inspect details.",
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
	_ = showAction.SetText("Show")
	showAction.Triggered().Attach(a.showFromTray)
	exitAction := walk.NewAction()
	_ = exitAction.SetText("Exit")
	exitAction.Triggered().Attach(func() { walk.App().Exit(0) })
	_ = ni.ContextMenu().Actions().Add(showAction)
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
	a.updateStatus(fmt.Sprintf("simple monitor running; %d USB devices; log: %s", len(devices), a.logger.Path()))
}

func (a *app) addEvent(event model.Event, persist bool) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	a.events.Add(event)
	if persist {
		_ = a.logger.Append(event)
	}
	if a.eventView != nil && a.events.RowCount() > 0 {
		_ = a.eventView.SetSelectedIndexes([]int{a.events.RowCount() - 1})
	}
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
	a.details.SetText("Select a device or event to inspect details.")
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
	a.statusLabel.SetText(fmt.Sprintf("%s | %s | v%s", text, admin, versionOrDev(a.cfg.Version)))
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
	dlg.Title = "Export visible JSONL log"
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
		_ = a.notifyIcon.ShowInfo("USB Suspend Watch", "Still monitoring USB devices in the tray.")
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

type deviceTableModel struct {
	walk.TableModelBase
	items []model.DeviceSnapshot
}

func newDeviceTableModel() *deviceTableModel {
	return &deviceTableModel{}
}

func (m *deviceTableModel) RowCount() int {
	return len(m.items)
}

func (m *deviceTableModel) Value(row, col int) interface{} {
	d := m.items[row]
	switch col {
	case 0:
		return d.DisplayName()
	case 1:
		return d.VIDPID()
	case 2:
		return string(d.PowerState)
	case 3:
		return d.Enumerator
	case 4:
		return d.Location
	default:
		return ""
	}
}

func (m *deviceTableModel) Set(items []model.DeviceSnapshot) {
	m.items = items
	m.PublishRowsReset()
}

func (m *deviceTableModel) Item(row int) (model.DeviceSnapshot, bool) {
	if row < 0 || row >= len(m.items) {
		return model.DeviceSnapshot{}, false
	}
	return m.items[row], true
}

type eventTableModel struct {
	walk.TableModelBase
	items []model.Event
	limit int
}

func newEventTableModel(limit int) *eventTableModel {
	return &eventTableModel{limit: limit}
}

func (m *eventTableModel) RowCount() int {
	return len(m.items)
}

func (m *eventTableModel) Value(row, col int) interface{} {
	e := m.items[row]
	switch col {
	case 0:
		return e.Time.Format("2006-01-02 15:04:05")
	case 1:
		return string(e.Type)
	case 2:
		return string(e.Confidence)
	case 3:
		return string(e.Source)
	case 4:
		return e.Device.DisplayName()
	case 5:
		return e.Message
	default:
		return ""
	}
}

func (m *eventTableModel) Add(event model.Event) {
	m.items = append(m.items, event)
	if m.limit > 0 && len(m.items) > m.limit {
		m.items = m.items[len(m.items)-m.limit:]
	}
	m.PublishRowsReset()
}

func (m *eventTableModel) Item(row int) (model.Event, bool) {
	if row < 0 || row >= len(m.items) {
		return model.Event{}, false
	}
	return m.items[row], true
}

func (m *eventTableModel) All() []model.Event {
	out := make([]model.Event, len(m.items))
	copy(out, m.items)
	return out
}

func formatDevice(d model.DeviceSnapshot) string {
	lines := []string{
		"Device",
		"Name: " + d.DisplayName(),
		"Instance ID: " + d.InstanceID,
		"Hardware ID: " + d.HardwareID,
		"VID/PID: " + d.VIDPID(),
		"Revision: " + d.Revision,
		"Serial: " + d.Serial,
		"Power state: " + string(d.PowerState),
		"Manufacturer: " + d.Manufacturer,
		"Service: " + d.Service,
		"Class: " + d.Class,
		"Enumerator: " + d.Enumerator,
		"Location: " + d.Location,
		"Last seen: " + d.LastSeen.Format(time.RFC3339),
	}
	return strings.Join(lines, "\r\n")
}

func formatEvent(e model.Event) string {
	lines := []string{
		"Event",
		"Time: " + e.Time.Format(time.RFC3339Nano),
		"Type: " + string(e.Type),
		"Source: " + string(e.Source),
		"Confidence: " + string(e.Confidence),
		"Message: " + e.Message,
		"Provider: " + e.Provider,
		fmt.Sprintf("Event ID: %d", e.EventID),
		"",
		formatDevice(e.Device),
	}
	if len(e.Raw) > 0 {
		lines = append(lines, "", "Raw ETW properties:")
		keys := make([]string, 0, len(e.Raw))
		for key := range e.Raw {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, key+": "+e.Raw[key])
		}
	}
	return strings.Join(lines, "\r\n")
}

func buildIcon() (*walk.Icon, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	bg := color.RGBA{R: 32, G: 48, B: 66, A: 255}
	accent := color.RGBA{R: 60, G: 190, B: 180, A: 255}
	hot := color.RGBA{R: 255, G: 190, B: 82, A: 255}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, bg)
		}
	}
	for y := 7; y < 25; y++ {
		for x := 13; x < 19; x++ {
			img.Set(x, y, accent)
		}
	}
	for y := 20; y < 26; y++ {
		for x := 8; x < 24; x++ {
			img.Set(x, y, accent)
		}
	}
	for y := 4; y < 9; y++ {
		for x := 11; x < 14; x++ {
			img.Set(x, y, hot)
		}
		for x := 18; x < 21; x++ {
			img.Set(x, y, hot)
		}
	}
	for y := 11; y < 14; y++ {
		for x := 20; x < 26; x++ {
			img.Set(x, y, hot)
		}
	}
	return walk.NewIconFromImageForDPI(img, 96)
}

func versionOrDev(v string) string {
	if v == "" {
		return "dev"
	}
	return v
}
