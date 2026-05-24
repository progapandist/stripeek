package history

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/progapandist/stripeek/proxy"
)

func TestStoreRotatesAndReloadsNewestCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.json")
	store := New(path, 2)

	for _, p := range []string{"/v1/oldest", "/v1/middle", "/v1/newest"} {
		if err := store.Append(proxy.Call{
			Time:   time.Now(),
			Method: "GET",
			Path:   p,
			Status: 200,
		}); err != nil {
			t.Fatalf("append %s: %v", p, err)
		}
	}

	loaded, err := New(path, 2).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len(loaded) = %d, want 2", len(loaded))
	}
	if loaded[0].Path != "/v1/middle" || loaded[1].Path != "/v1/newest" {
		t.Fatalf("loaded paths = %q, %q", loaded[0].Path, loaded[1].Path)
	}
}

func TestStoreDisabledWhenLimitIsZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.json")
	store := New(path, 0)

	if err := store.Append(proxy.Call{Path: "/v1/customers"}); err != nil {
		t.Fatalf("append with disabled store: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load with disabled store: %v", err)
	}
	if loaded != nil {
		t.Fatalf("loaded = %#v, want nil", loaded)
	}
}
