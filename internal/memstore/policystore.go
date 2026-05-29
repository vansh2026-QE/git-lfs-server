package memstore

import (
	"context"
	"os"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// StringPolicyStore serves a fixed byte slice. Watch is unsupported, so it
// returns a nil channel: a nil channel never fires when received on, so the
// Loader transparently skips hot reload. Used in tests.
// See docs/auth-design.md §4.3.
type StringPolicyStore struct {
	content []byte
}

// NewStringPolicyStore returns a store that always serves s.
func NewStringPolicyStore(s string) *StringPolicyStore {
	return &StringPolicyStore{content: []byte(s)}
}

// Load returns a copy so callers cannot mutate the store's backing bytes.
func (s *StringPolicyStore) Load(context.Context) ([]byte, error) {
	out := make([]byte, len(s.content))
	copy(out, s.content)
	return out, nil
}

// Watch reports no change-notification support via a nil channel.
func (s *StringPolicyStore) Watch(context.Context) (<-chan struct{}, error) {
	return nil, nil
}

// FilePolicyStore reads policy bytes from a filesystem path. Watch polls the
// file's stat fingerprint (mtime + size) on an interval and emits whenever it
// changes. See docs/auth-design.md §4.3.
type FilePolicyStore struct {
	path     string
	interval time.Duration
}

const defaultPollInterval = 250 * time.Millisecond

// NewFilePolicyStore returns a store backed by the file at path.
func NewFilePolicyStore(path string) *FilePolicyStore {
	return &FilePolicyStore{path: path, interval: defaultPollInterval}
}

// Load reads the whole file. A read error is an infrastructure error, not a
// "policy malformed" verdict; the Loader owns parsing.
func (f *FilePolicyStore) Load(context.Context) ([]byte, error) {
	return os.ReadFile(f.path)
}

// Watch starts a polling goroutine and returns its notification channel. The
// goroutine stops and the channel closes when ctx is cancelled.
//
// The baseline fingerprint is captured synchronously here, before the
// goroutine starts, so a change made immediately after Watch returns is not
// missed by a late-scheduled poller.
func (f *FilePolicyStore) Watch(ctx context.Context) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	go f.poll(ctx, ch, f.fingerprint())
	return ch, nil
}

func (f *FilePolicyStore) poll(ctx context.Context, ch chan<- struct{}, last statFingerprint) {
	defer close(ch)
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cur := f.fingerprint()
			if !cur.mod.Equal(last.mod) || cur.size != last.size {
				last = cur
				// Non-blocking send: if no one is listening, drop the
				// signal rather than stall the poller. Buffer of 1
				// coalesces bursts into a single pending notification.
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}
}

type statFingerprint struct {
	mod  time.Time
	size int64
}

func (f *FilePolicyStore) fingerprint() statFingerprint {
	fi, err := os.Stat(f.path)
	if err != nil {
		return statFingerprint{}
	}
	return statFingerprint{mod: fi.ModTime(), size: fi.Size()}
}

var (
	_ ports.PolicyStore = (*StringPolicyStore)(nil)
	_ ports.PolicyStore = (*FilePolicyStore)(nil)
)
