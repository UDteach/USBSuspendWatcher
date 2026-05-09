package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"usb-suspend-watch/internal/model"
)

type EventLogger struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func ResolveLogDir() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exe), "logs")
		if mkErr := os.MkdirAll(dir, 0755); mkErr == nil {
			probe := filepath.Join(dir, ".write-test")
			if writeErr := os.WriteFile(probe, []byte("ok"), 0644); writeErr == nil {
				_ = os.Remove(probe)
				return dir, nil
			}
		}
	}

	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = os.TempDir()
	}
	dir := filepath.Join(local, "UsbSuspendWatch", "logs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func NewEventLogger(dir string) (*EventLogger, error) {
	return NewEventLoggerWithPrefix(dir, "usb-suspend-watch")
}

func NewEventLoggerWithPrefix(dir, prefix string) (*EventLogger, error) {
	if prefix == "" {
		prefix = "usb-suspend-watch"
	}
	path := PathForPrefix(dir, prefix, time.Now())
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &EventLogger{path: path, file: f}, nil
}

func PathForPrefix(dir, prefix string, t time.Time) string {
	if prefix == "" {
		prefix = "usb-suspend-watch"
	}
	name := fmt.Sprintf("%s-%s.jsonl", prefix, t.Format("20060102"))
	return filepath.Join(dir, name)
}

func (l *EventLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *EventLogger) Append(event model.Event) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := l.file.Write(append(b, '\n')); err != nil {
		return err
	}
	return l.file.Sync()
}

func (l *EventLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func ExportEvents(path string, events []model.Event) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, event := range events {
		b, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return w.Flush()
}

func ReadEventsSince(path string, offset int64) ([]model.Event, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, offset, nil
		}
		return nil, offset, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, offset, err
	}
	var events []model.Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			events = append(events, event)
		}
	}
	pos, _ := f.Seek(0, 1)
	return events, pos, scanner.Err()
}
