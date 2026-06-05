package qe

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/token"
)

// revealClient is the slice of Client reveal needs: the authorized real paths
// and a way to stream an object's bytes. Narrowing to an interface keeps Reveal
// testable with a fake and free of HTTP concerns.
type revealClient interface {
	Names(ctx context.Context) ([]string, error)
	Content(ctx context.Context, oid string, w io.Writer) error
}

// Reveal projects the hashed repo's authorized subtree into the legible
// workspace. For every real path the server authorizes (/names), it computes the
// token path, reads that token-named pointer stub from the hashed working tree,
// dereferences the pointer's bytes via lfsd, and writes them to the real path in
// legible. Unauthorized files are simply never visited, so they stay absent from
// legible (their stubs remain only in the hashed repo). See §3, §7.2, §9.
//
// Reveal is best-effort across paths: a per-path failure (missing stub,
// unparsable pointer, fetch error) is collected and reporting continues, so one
// bad object does not abort the whole projection. The aggregate error, if any,
// is returned after all paths are attempted.
func Reveal(ctx context.Context, c revealClient, l Layout) error {
	paths, err := c.Names(ctx)
	if err != nil {
		return err
	}
	var failures []string
	for _, real := range paths {
		if err := revealOne(ctx, c, l, real); err != nil {
			failures = append(failures, fmt.Sprintf("  %s: %v", real, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("qe: reveal failed for %d path(s):\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
	return nil
}

// revealOne reveals a single authorized real path: token -> stub -> oid -> bytes.
func revealOne(ctx context.Context, c revealClient, l Layout, real string) error {
	tok, err := token.Path(real)
	if err != nil {
		return err
	}
	stub, err := os.ReadFile(filepath.Join(l.Hashed(), tok))
	if err != nil {
		return fmt.Errorf("read stub: %w", err)
	}
	ptr, err := ParsePointer(stub)
	if err != nil {
		return err
	}
	dest := filepath.Join(l.Legible(), filepath.FromSlash(real))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if err := c.Content(ctx, ptr.OID, f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
