package etwhelper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tekert/goetw/etw"
	"golang.org/x/sys/windows"

	"usb-suspend-watch/internal/logs"
	"usb-suspend-watch/internal/model"
)

type Config struct {
	LogDir    string
	Session   string
	StopFile  string
	ParentPID int
}

func Run(cfg Config) int {
	if cfg.Session == "" {
		cfg.Session = defaultSessionName()
	}
	if cfg.LogDir == "" {
		dir, err := logs.ResolveLogDir()
		if err != nil {
			return 2
		}
		cfg.LogDir = dir
	}
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return 2
	}

	logger, err := logs.NewEventLoggerWithPrefix(cfg.LogDir, "usb-suspend-watch-etw")
	if err != nil {
		return 2
	}
	defer logger.Close()
	appendEvent := func(event model.Event) {
		_ = logger.Append(event)
	}
	appendEvent(model.Event{
		Time:       time.Now(),
		Type:       model.EventInfo,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceHigh,
		Message:    "ETW helper starting",
	})
	parentWatch, err := openParentWatch(cfg.ParentPID)
	if err != nil {
		appendEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventInfo,
			Source:     model.SourceApp,
			Confidence: model.ConfidenceLow,
			Message:    "ETW helper could not watch parent process: " + err.Error(),
		})
	}
	if parentWatch != nil {
		defer parentWatch.Close()
	}

	session, enabledProviders, err := startUSBSession(cfg.Session, appendEvent)
	if err != nil {
		appendEvent(errorEvent("start ETW session: " + err.Error()))
		return 3
	}
	defer session.Stop()
	if os.Getenv("USB_SUSPEND_WATCH_ETW_RUNDOWN") == "1" {
		_ = session.GetRundownEvents(nil)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	consumer := etw.NewConsumer(ctx)
	consumer.FromSessions(session)
	go func() {
		_ = consumer.ProcessEvents(func(e *etw.Event) {
			appendEvent(model.NormalizeETW(toRecord(e)))
		})
	}()
	if err := consumer.Start(); err != nil {
		appendEvent(errorEvent("start ETW consumer: " + err.Error()))
		cancel()
		return 3
	}
	defer consumer.Stop()

	appendEvent(model.Event{
		Time:       time.Now(),
		Type:       model.EventInfo,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceHigh,
		Message:    "ETW helper running",
		Raw:        map[string]string{"providers": strings.Join(enabledProviders, ",")},
	})

	for {
		if parentWatch != nil && parentWatch.Exited() {
			appendEvent(model.Event{
				Time:       time.Now(),
				Type:       model.EventInfo,
				Source:     model.SourceApp,
				Confidence: model.ConfidenceHigh,
				Message:    "ETW helper parent process exited",
			})
			cancel()
			return 0
		}
		if cfg.StopFile != "" {
			if _, err := os.Stat(cfg.StopFile); err == nil {
				appendEvent(model.Event{
					Time:       time.Now(),
					Type:       model.EventInfo,
					Source:     model.SourceApp,
					Confidence: model.ConfidenceHigh,
					Message:    "ETW helper stop file detected",
				})
				cancel()
				return 0
			}
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(1 * time.Second):
		}
	}
}

func startUSBSession(name string, appendEvent func(model.Event)) (*etw.RealTimeSession, []string, error) {
	session := etw.NewRealTimeSession(name)
	if err := session.Start(); err != nil {
		return nil, nil, err
	}

	var enabled []string
	for _, provider := range providers() {
		if err := session.EnableProvider(provider); err != nil {
			appendEvent(model.Event{
				Time:       time.Now(),
				Type:       model.EventInfo,
				Source:     model.SourceApp,
				Confidence: model.ConfidenceLow,
				Message:    "ETW provider unavailable: " + provider.Name + ": " + err.Error(),
			})
			continue
		}
		enabled = append(enabled, provider.Name)
		appendEvent(model.Event{
			Time:       time.Now(),
			Type:       model.EventInfo,
			Source:     model.SourceApp,
			Confidence: model.ConfidenceHigh,
			Message:    "ETW provider enabled: " + provider.Name,
		})
	}

	if len(enabled) == 0 {
		_ = session.Stop()
		return nil, nil, fmt.Errorf("no USB ETW providers could be enabled")
	}
	return session, enabled, nil
}

type parentWatch struct {
	handle windows.Handle
}

func openParentWatch(pid int) (*parentWatch, error) {
	if pid <= 0 {
		return nil, nil
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil, err
	}
	return &parentWatch{handle: handle}, nil
}

func (w *parentWatch) Exited() bool {
	if w == nil || w.handle == 0 {
		return false
	}
	result, err := windows.WaitForSingleObject(w.handle, 0)
	return err == nil && result == windows.WAIT_OBJECT_0
}

func (w *parentWatch) Close() {
	if w == nil || w.handle == 0 {
		return
	}
	_ = windows.CloseHandle(w.handle)
	w.handle = 0
}

func providers() []etw.Provider {
	usbPowerKeywords := usbProviderKeywords()
	return []etw.Provider{
		provider("Microsoft-Windows-USB-USBHUB3", usbPowerKeywords),
		provider("Microsoft-Windows-USB-UCX", usbPowerKeywords),
		provider("Microsoft-Windows-USB-USBXHCI", usbPowerKeywords),
	}
}

func usbProviderKeywords() uint64 {
	const powerKeyword = 0x8
	const rundownKeyword = 0x8000
	if os.Getenv("USB_SUSPEND_WATCH_ETW_RUNDOWN") == "1" {
		return powerKeyword | rundownKeyword
	}
	return powerKeyword
}

func provider(name string, keywords uint64) etw.Provider {
	knownGUIDs := map[string]string{
		"Microsoft-Windows-USB-USBHUB3": "{ac52ad17-cc01-4f85-8df5-4dce4333c99b}",
		"Microsoft-Windows-USB-UCX":     "{36da592d-e43a-4e28-af6f-4bc57c5a11e8}",
		"Microsoft-Windows-USB-USBXHCI": "{30e1d284-5d88-459c-83fd-6345b39b19ec}",
	}
	if guid := knownGUIDs[name]; guid != "" {
		return providerFromGUID(name, guid, keywords, 0)
	}
	provider, err := etw.ParseProvider(fmt.Sprintf("%s:0xff::0x%x", name, keywords))
	if err == nil {
		provider.EnableProperties = 0
	}
	return provider
}

func providerFromGUID(name, guid string, keywords uint64, enableProperties uint32) etw.Provider {
	return etw.Provider{
		GUID:             *etw.MustParseGUID(guid),
		Name:             name,
		EnableLevel:      0xff,
		MatchAnyKeyword:  keywords,
		MatchAllKeyword:  0,
		EnableProperties: enableProperties,
	}
}

func toRecord(e *etw.Event) model.ETWRecord {
	props := make(map[string]string, len(e.EventData)+len(e.UserData)+3)
	for _, p := range e.EventData {
		props[p.Name] = fmt.Sprint(p.Value)
	}
	for _, p := range e.UserData {
		props[p.Name] = fmt.Sprint(p.Value)
	}
	return model.ETWRecord{
		Time:       e.System.TimeCreated.SystemTime,
		Provider:   e.System.Provider.Name,
		EventID:    e.System.EventID,
		Task:       e.System.Task.Name,
		Opcode:     e.System.Opcode.Name,
		Properties: props,
	}
}

func errorEvent(message string) model.Event {
	return model.Event{
		Time:       time.Now(),
		Type:       model.EventError,
		Source:     model.SourceApp,
		Confidence: model.ConfidenceLow,
		Message:    message,
	}
}

func defaultSessionName() string {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
	return name + "-etw"
}
