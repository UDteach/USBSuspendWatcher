package etwhelper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tekert/goetw/etw"

	"usb-suspend-watch/internal/logs"
	"usb-suspend-watch/internal/model"
)

type Config struct {
	LogDir   string
	Session  string
	StopFile string
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

	session := etw.NewRealTimeSession(cfg.Session)
	for _, provider := range providers() {
		if err := session.AddProvider(provider); err != nil {
			appendEvent(errorEvent("add ETW provider: " + err.Error()))
			return 3
		}
	}
	if err := session.Start(); err != nil {
		appendEvent(errorEvent("start ETW session: " + err.Error()))
		return 3
	}
	defer session.Stop()
	_ = session.GetRundownEvents(nil)

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
	})

	for {
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

func providers() []etw.Provider {
	const usbKeywords = 0xffffffff
	return []etw.Provider{
		provider("Microsoft-Windows-USB-USBHUB3", usbKeywords),
		provider("Microsoft-Windows-USB-UCX", usbKeywords),
		provider("Microsoft-Windows-USB-USBXHCI", usbKeywords),
	}
}

func provider(name string, keywords uint64) etw.Provider {
	if name == "Microsoft-Windows-USB-USBXHCI" {
		return providerFromGUID(name, "{30e1d284-5d88-459c-83fd-6345b39b19ec}", keywords, 0)
	}
	provider, err := etw.ParseProvider(fmt.Sprintf("%s:0xff::0x%x", name, keywords))
	if err == nil {
		return provider
	}
	knownGUIDs := map[string]string{
		"Microsoft-Windows-USB-USBHUB3": "{ac52ad17-cc01-4f85-8df5-4dce4333c99b}",
		"Microsoft-Windows-USB-UCX":     "{36da592d-e43a-4e28-af6f-4bc57c5a11e8}",
		"Microsoft-Windows-USB-USBXHCI": "{30e1d284-5d88-459c-83fd-6345b39b19ec}",
	}
	return providerFromGUID(name, knownGUIDs[name], keywords, etw.EVENT_ENABLE_PROPERTY_PROCESS_START_KEY)
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
