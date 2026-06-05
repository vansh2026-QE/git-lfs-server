package qe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/token"
)

func TestSyncInCopiesToTokenPath(t *testing.T) {
	l, err := NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	real := "private/privBob/secret.txt"
	want := "the real bytes\n"
	mustWrite(t, filepath.Join(l.Legible(), filepath.FromSlash(real)), want)

	if err := SyncIn(l, []string{real}); err != nil {
		t.Fatalf("SyncIn: %v", err)
	}

	tok, err := token.Path(real)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(l.Hashed(), filepath.FromSlash(tok)))
	if err != nil {
		t.Fatalf("read hashed token file: %v", err)
	}
	if string(got) != want {
		t.Errorf("hashed bytes = %q, want %q", string(got), want)
	}
}

func TestSyncInMissingSourceErrors(t *testing.T) {
	l, err := NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	if err := SyncIn(l, []string{"nope/missing.txt"}); err == nil {
		t.Fatal("expected error for missing legible source")
	}
}
