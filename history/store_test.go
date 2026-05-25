package history

import (
	"path/filepath"
	"sync"
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

func TestStorePersistsRequestGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.json")
	started := time.Now().UTC().Truncate(time.Second)
	group := &proxy.Group{
		ID:        "group-123",
		Name:      "Group Teal",
		Color:     "Teal",
		LightHex:  "#0f766e",
		DarkHex:   "#5eead4",
		StartedAt: started,
	}
	if err := New(path, 10).Append(proxy.Call{
		Time:   time.Now(),
		Method: "GET",
		Path:   "/v1/customers",
		Status: 200,
		Group:  group,
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	loaded, err := New(path, 10).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].Group == nil || loaded[0].Group.ID != group.ID || loaded[0].Group.Name != group.Name {
		t.Fatalf("loaded group = %#v, want %#v", loaded[0].Group, group)
	}
}

func TestStoreSerializesConcurrentAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calls.json")
	store := New(path, 20)

	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.Append(proxy.Call{
				Time:   time.Now(),
				Method: "GET",
				Path:   "/v1/customers",
				Status: 200,
			})
		}()

		if i%4 == 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs <- store.Clear()
			}()
		}
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent store operation: %v", err)
		}
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("load after concurrent operations: %v", err)
	}
}
