package qe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutPaths(t *testing.T) {
	l, err := NewLayout("/tmp/work")
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	cases := map[string]string{
		l.Legible():    "/tmp/work",
		l.QeDir():      "/tmp/work/.qe",
		l.Hashed():     "/tmp/work/.qe/hashed",
		l.ConfigPath(): "/tmp/work/.qe/config",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	}
}

func TestNewLayoutResolvesRelative(t *testing.T) {
	l, err := NewLayout("work")
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	if !filepath.IsAbs(l.Root) {
		t.Errorf("Root = %q, want absolute", l.Root)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	l, err := NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	want := Config{
		Server: "http://localhost:8080",
		Repo:   "demo",
		User:   "bob",
		LFSURL: "http://bob:bobpw@localhost:8080/demo",
	}
	if err := l.WriteConfig(want); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	got, err := l.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestReadConfigIgnoresBlankAndComments(t *testing.T) {
	l, err := NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	if err := writeRaw(l.ConfigPath(), "# a comment\n\nrepo=demo\n  \nunknown=ignored\nuser=bob\n"); err != nil {
		t.Fatal(err)
	}
	got, err := l.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if got.Repo != "demo" || got.User != "bob" {
		t.Errorf("got %+v, want repo=demo user=bob", got)
	}
}

func TestReadConfigMalformedLine(t *testing.T) {
	l, err := NewLayout(t.TempDir())
	if err != nil {
		t.Fatalf("NewLayout: %v", err)
	}
	if err := writeRaw(l.ConfigPath(), "this-has-no-equals\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ReadConfig(); err == nil {
		t.Fatal("expected error for malformed config line")
	}
}

// writeRaw drops bytes at path, creating parent dirs, for tests that need a
// hand-crafted config file rather than WriteConfig's canonical output.
func writeRaw(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}
