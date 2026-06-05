package qe

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/token"
)

// BuildSidecar derives the token->real path map from the legible workspace: for
// every real file the human owns, it records token.Path(real) -> real. The map
// is the forward direction (our own outgoing paths), computed on the fly from
// legible and never persisted (docs/name-hiding-design.md §7.3). commit hands it
// to the redaction hook so a staged token path can be bound to its real path.
//
// Real paths are slash-separated and relative to legible (matching token.Path /
// git path style). The .qe control directory is skipped; legible is not itself a
// git repo, so there is no .git to exclude.
func BuildSidecar(l Layout) (map[string]string, error) {
	root := l.Legible()
	m := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == l.QeDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		real := filepath.ToSlash(rel)
		tok, err := token.Path(real)
		if err != nil {
			return fmt.Errorf("qe: tokenize %q: %w", real, err)
		}
		m[tok] = real
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("qe: build sidecar: %w", err)
	}
	return m, nil
}

// WriteSidecar writes m to a temp file as deterministic "tokenpath\trealpath"
// lines and returns its path. The caller exports QEGIT_SIDECAR=<path> for the
// git commit it runs (so the commit-msg hook inherits it) and removes the file
// afterward; the sidecar is per-command, never persisted (§7.3).
func WriteSidecar(m map[string]string) (string, error) {
	toks := make([]string, 0, len(m))
	for tok := range m {
		toks = append(toks, tok)
	}
	sort.Strings(toks)
	var b strings.Builder
	for _, tok := range toks {
		fmt.Fprintf(&b, "%s\t%s\n", tok, m[tok])
	}
	f, err := os.CreateTemp("", "qegit-sidecar-*")
	if err != nil {
		return "", fmt.Errorf("qe: create sidecar: %w", err)
	}
	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("qe: write sidecar: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("qe: close sidecar: %w", err)
	}
	return f.Name(), nil
}
