package portstest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// RunPolicyStoreContract exercises the ports.PolicyStore contract. The
// factory returns a fresh store preloaded with the given content.
//
// Watch is optional: an implementation may return a nil channel to signal
// "no hot reload". The suite accepts that; when the channel is non-nil it
// must close after ctx is cancelled.
//
// See docs/auth-design.md §4.3.
func RunPolicyStoreContract(t *testing.T, factory func(content []byte) ports.PolicyStore) {
	t.Helper()

	t.Run("LoadReturnsContent", func(t *testing.T) {
		want := []byte(`{"version":1}`)
		got, err := factory(want).Load(context.Background())
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if string(got) != string(want) {
			t.Errorf("Load = %q, want %q", got, want)
		}
	})

	t.Run("LoadIsRepeatable", func(t *testing.T) {
		want := []byte(`{"version":1}`)
		store := factory(want)
		for i := 0; i < 3; i++ {
			got, err := store.Load(context.Background())
			if err != nil {
				t.Fatalf("Load %d: %v", i, err)
			}
			if string(got) != string(want) {
				t.Errorf("Load %d = %q, want %q", i, got, want)
			}
		}
	})

	t.Run("ConcurrentLoadsAreSafe", func(t *testing.T) {
		store := factory([]byte(`{"version":1}`))
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = store.Load(context.Background())
			}()
		}
		wg.Wait()
	})

	t.Run("WatchNilOrClosesOnCancel", func(t *testing.T) {
		store := factory([]byte(`{"version":1}`))
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := store.Watch(ctx)
		if err != nil {
			t.Fatalf("Watch: %v", err)
		}
		if ch == nil {
			cancel()
			return // nil channel: hot reload unsupported, valid per contract
		}
		cancel()
		select {
		case <-ch:
			// Closed (or a final signal then close): acceptable.
		case <-time.After(2 * time.Second):
			t.Error("Watch channel did not close within 2s of ctx cancel")
		}
	})
}
