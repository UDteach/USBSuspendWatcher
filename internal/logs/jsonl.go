package logs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type JSONLWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func NewJSONLWriter(path string) (*JSONLWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &JSONLWriter{path: path, file: f}, nil
}

func (w *JSONLWriter) Path() string {
	if w == nil {
		return ""
	}
	return w.path
}

func (w *JSONLWriter) Append(v interface{}) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.file.Write(append(b, '\n')); err != nil {
		return err
	}
	return w.file.Sync()
}

func (w *JSONLWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	return w.file.Close()
}
