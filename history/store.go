package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/progapandist/stripeek/proxy"
)

// Store keeps a rotating JSON history of captured calls.
type Store struct {
	path  string
	limit int
	calls []proxy.Call
}

func New(path string, limit int) *Store {
	return &Store{path: path, limit: limit}
}

func DefaultPath() string {
	return filepath.Join(os.TempDir(), "stripeek-calls.json")
}

func (s *Store) Load() ([]proxy.Call, error) {
	if s.limit <= 0 {
		s.calls = nil
		return nil, nil
	}

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.calls = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		s.calls = nil
		return nil, nil
	}

	var calls []proxy.Call
	if err := json.Unmarshal(b, &calls); err != nil {
		return nil, err
	}
	s.calls = trimOldest(calls, s.limit)
	return append([]proxy.Call(nil), s.calls...), nil
}

func (s *Store) Clear() error {
	s.calls = nil
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) Append(c proxy.Call) error {
	if s.limit <= 0 {
		return nil
	}
	s.calls = append(s.calls, c)
	s.calls = trimOldest(s.calls, s.limit)
	return s.flush()
}

func (s *Store) flush() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".stripeek-calls-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.calls); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func trimOldest(calls []proxy.Call, limit int) []proxy.Call {
	if limit <= 0 || len(calls) <= limit {
		return calls
	}
	return calls[len(calls)-limit:]
}
