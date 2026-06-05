package qe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/token"
)

func TestBuildSidecarMapsTokenToReal(t *testing.T) {
	l, err := NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	reals := []string{"public/hello.txt", "private/privBob/secret.txt"}
	for _, r := range reals {
		mustWrite(t, filepath.Join(l.Legible(), filepath.FromSlash(r)), "x")
	}
	// .qe content must never enter the map.
	mustWrite(t, filepath.Join(l.Hashed(), "config"), "noise")

	m, err := BuildSidecar(l)
	if err != nil {
		t.Fatalf("BuildSidecar: %v", err)
	}
	if len(m) != len(reals) {
		t.Fatalf("map has %d entries, want %d: %v", len(m), len(reals), m)
	}
	for _, r := range reals {
		tok, err := token.Path(r)
		if err != nil {
			t.Fatal(err)
		}
		if got := m[tok]; got != r {
			t.Errorf("m[token(%q)] = %q, want %q", r, got, r)
		}
	}
}

func TestWriteSidecarFormatAndRoundTrip(t *testing.T) {
	m := map[string]string{
		"aaa/bbb": "public/hello.txt",
		"ccc":     "top.txt",
	}
	path, err := WriteSidecar(m)
	if err != nil {
		t.Fatalf("WriteSidecar: %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	want := "aaa/bbb\tpublic/hello.txt\nccc\ttop.txt\n"
	if string(data) != want {
		t.Errorf("sidecar = %q, want %q", string(data), want)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
