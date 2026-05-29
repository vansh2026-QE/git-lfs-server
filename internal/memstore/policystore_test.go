package memstore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/ports"
	"github.com/vansh2026/git-lfs-server/internal/ports/portstest"
)

func TestStringPolicyStore_Contract(t *testing.T) {
	portstest.RunPolicyStoreContract(t, func(content []byte) ports.PolicyStore {
		return memstore.NewStringPolicyStore(string(content))
	})
}

func TestFilePolicyStore_Contract(t *testing.T) {
	portstest.RunPolicyStoreContract(t, func(content []byte) ports.PolicyStore {
		path := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		return memstore.NewFilePolicyStore(path)
	})
}

// File-specific: Watch emits when the file changes. The rewrite uses a
// different length so detection does not depend on mtime resolution.
func TestFilePolicyStore_WatchEmitsOnChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"v":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := memstore.NewFilePolicyStore(path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := store.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"version":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Error("expected Watch signal after file change, got none")
	}
}
