// Package qe implements the qegit name-hiding client: the two-directory (.qe)
// model from docs/name-hiding-design.md §7. A legible workspace of real names
// and real bytes sits atop a hashed git repo of token-named LFS pointer stubs.
// This file owns the on-disk layout and the small .qe/config key=value file.
package qe

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Layout resolves the fixed paths of the two-directory model rooted at a legible
// workspace directory. The legible workspace itself is Root (real names, real
// bytes, not a git repo); the hashed git repo and config live under Root/.qe.
// See docs/name-hiding-design.md §7.1.
type Layout struct {
	Root string // absolute path to the legible workspace
}

// NewLayout returns a Layout rooted at the legible workspace dir, resolved to an
// absolute path so later commands are insensitive to the process working dir.
func NewLayout(root string) (Layout, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Layout{}, fmt.Errorf("qe: resolve root %q: %w", root, err)
	}
	return Layout{Root: abs}, nil
}

// Legible is the real-named workspace the human edits.
func (l Layout) Legible() string { return l.Root }

// QeDir is the hidden control directory holding the hashed repo and config.
func (l Layout) QeDir() string { return filepath.Join(l.Root, ".qe") }

// Hashed is the real git repo: token-named stubs plus the blob cache in .git.
func (l Layout) Hashed() string { return filepath.Join(l.QeDir(), "hashed") }

// ConfigPath is the .qe/config key=value file.
func (l Layout) ConfigPath() string { return filepath.Join(l.QeDir(), "config") }

// Config is the persisted per-workspace state: where the policy server lives,
// which repo and user this clone speaks for, and the full lfs.url (with creds)
// the client dereferences pointers through. It is deliberately small; the
// token<->real sidecar map is derived per-command, never persisted (§7.3).
type Config struct {
	Server string // lfsd origin, e.g. http://localhost:8080
	Repo   string // policy repo name in the LFS URL path, e.g. demo
	User   string // LFS user this workspace acts as, e.g. bob
	LFSURL string // full lfs.url incl. userinfo, e.g. http://bob:bobpw@localhost:8080/demo
}

// WriteConfig writes cfg to .qe/config as deterministic key=value lines,
// creating the .qe directory if needed.
func (l Layout) WriteConfig(cfg Config) error {
	if err := os.MkdirAll(l.QeDir(), 0o755); err != nil {
		return fmt.Errorf("qe: create %q: %w", l.QeDir(), err)
	}
	fields := map[string]string{
		"server":  cfg.Server,
		"repo":    cfg.Repo,
		"user":    cfg.User,
		"lfs.url": cfg.LFSURL,
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, fields[k])
	}
	if err := os.WriteFile(l.ConfigPath(), []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("qe: write config %q: %w", l.ConfigPath(), err)
	}
	return nil
}

// ReadConfig parses .qe/config. Blank lines and '#' comments are ignored;
// unknown keys are tolerated so the format can grow without breaking old
// clients. Values keep everything after the first '=' verbatim (creds in
// lfs.url may contain characters that are not worth re-escaping here).
func (l Layout) ReadConfig() (Config, error) {
	f, err := os.Open(l.ConfigPath())
	if err != nil {
		return Config{}, fmt.Errorf("qe: open config %q: %w", l.ConfigPath(), err)
	}
	defer f.Close()

	var cfg Config
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return Config{}, fmt.Errorf("qe: malformed config line %q", line)
		}
		switch strings.TrimSpace(key) {
		case "server":
			cfg.Server = val
		case "repo":
			cfg.Repo = val
		case "user":
			cfg.User = val
		case "lfs.url":
			cfg.LFSURL = val
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, fmt.Errorf("qe: read config %q: %w", l.ConfigPath(), err)
	}
	return cfg, nil
}
