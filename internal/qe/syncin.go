package qe

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vansh2026/git-lfs-server/internal/token"
)

// SyncIn copies real bytes from the legible workspace into the hashed working
// tree at their token paths: for each real path, legible/<real> is written to
// hashed/<token.Path(real)>, creating parent dirs. This is the "sync in" half of
// the per-command pattern (docs/name-hiding-design.md §7.2); a subsequent
// `git add <tokenpath>` lets the LFS clean filter turn the bytes into a pointer
// stub. Real paths are slash-separated and relative to legible.
func SyncIn(l Layout, realPaths []string) error {
	for _, real := range realPaths {
		tok, err := token.Path(real)
		if err != nil {
			return fmt.Errorf("qe: tokenize %q: %w", real, err)
		}
		src := filepath.Join(l.Legible(), filepath.FromSlash(real))
		dst := filepath.Join(l.Hashed(), filepath.FromSlash(tok))
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("qe: sync in %q: %w", real, err)
		}
	}
	return nil
}

// copyFile copies src to dst, creating dst's parent directories.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
