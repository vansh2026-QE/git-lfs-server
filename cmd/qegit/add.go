package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/qe"
	"github.com/vansh2026/git-lfs-server/internal/token"
)

// runAdd stages real files into the hashed repo. Args are paths relative to the
// current directory (git-style); a directory (including ".") is expanded to the
// files it contains. Each real path's bytes are synced into the hashed tree at
// its token path, then `git add <tokenpath>` lets the LFS clean filter stage a
// pointer stub. This is the "sync in -> git add" half of the per-command
// pattern (design §7.2).
func runAdd(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("add: at least one PATH required")
	}
	layout, err := layoutFromCwd()
	if err != nil {
		return err
	}

	var reals []string
	for _, arg := range argv {
		got, err := collectFiles(layout, arg)
		if err != nil {
			return err
		}
		reals = append(reals, got...)
	}
	if len(reals) == 0 {
		return fmt.Errorf("add: no files to add")
	}

	tokens := make([]string, len(reals))
	for i, real := range reals {
		tok, err := token.Path(real)
		if err != nil {
			return fmt.Errorf("add: tokenize %q: %w", real, err)
		}
		tokens[i] = tok
	}

	// Only sync and stage paths whose legible bytes differ from the staged
	// pointer, so `add .` does not rewrite every token blob (design note: the
	// hashed working tree is not the change signal; the index is).
	reals, tokens, err = changedPaths(layout, reals, tokens)
	if err != nil {
		return err
	}
	if len(reals) == 0 {
		return nil
	}

	if err := qe.SyncIn(layout, reals); err != nil {
		return err
	}
	return runCommand(layout.Hashed(), nil, "git", append([]string{"add", "--"}, tokens...)...)
}

// collectFiles resolves a user-supplied path (relative to the current dir) into
// real paths relative to the legible root. A file yields itself; a directory is
// walked, yielding every contained file. The .qe control dir is always skipped,
// and paths resolving outside the workspace are rejected.
func collectFiles(layout qe.Layout, arg string) ([]string, error) {
	abs, err := filepath.Abs(arg)
	if err != nil {
		return nil, err
	}
	root := layout.Legible()
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("add: %q is outside the workspace", arg)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{filepath.ToSlash(rel)}, nil
	}

	var reals []string
	err = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == layout.QeDir() {
				return filepath.SkipDir
			}
			return nil
		}
		r, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		reals = append(reals, filepath.ToSlash(r))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reals, nil
}
