package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/qe"
)

func TestCollectFilesExpandsDotAndSkipsQe(t *testing.T) {
	layout, err := qe.NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	write(t, filepath.Join(layout.Legible(), "top.txt"))
	write(t, filepath.Join(layout.Legible(), "public", "hello.txt"))
	write(t, filepath.Join(layout.Hashed(), "noise")) // under .qe, must be skipped

	t.Chdir(layout.Legible())
	got, err := collectFiles(layout, ".")
	if err != nil {
		t.Fatalf("collectFiles: %v", err)
	}
	sort.Strings(got)
	want := []string{"public/hello.txt", "top.txt"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("collectFiles(.) = %v, want %v", got, want)
	}
}

func TestCollectFilesSubdirRelativeToCwd(t *testing.T) {
	layout, err := qe.NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	write(t, filepath.Join(layout.Legible(), "public", "hello.txt"))

	t.Chdir(filepath.Join(layout.Legible(), "public"))
	got, err := collectFiles(layout, "hello.txt")
	if err != nil {
		t.Fatalf("collectFiles: %v", err)
	}
	if len(got) != 1 || got[0] != "public/hello.txt" {
		t.Errorf("collectFiles = %v, want [public/hello.txt]", got)
	}
}

func TestCollectFilesRejectsOutsideWorkspace(t *testing.T) {
	layout, err := qe.NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	t.Chdir(layout.Legible())
	if _, err := collectFiles(layout, ".."); err == nil {
		t.Fatal("expected error for path outside workspace")
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
