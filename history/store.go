package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/progapandist/stripeek/proxy"
)

// Store keeps a rotating JSON history of captured calls. It is safe to use from
// the capture goroutine and the TUI update loop concurrently.
type Store struct {
	mu    sync.Mutex
	path  string
	limit int
	calls []proxy.Call
}

// New returns a Store backed by path. A non-positive limit disables persistence.
func New(path string, limit int) *Store {
	return &Store{path: path, limit: limit}
}

// DefaultPath returns the per-machine fallback history path.
func DefaultPath() string {
	return filepath.Join(os.TempDir(), "stripeek-calls.json")
}

// Load reads persisted calls from disk and returns them oldest first.
func (s *Store) Load() ([]proxy.Call, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// Clear removes in-memory and on-disk history.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = nil
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Append records c and rewrites the bounded history file.
func (s *Store) Append(c proxy.Call) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.limit <= 0 {
		return nil
	}
	s.calls = append(s.calls, c)
	s.calls = trimOldest(s.calls, s.limit)
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
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
