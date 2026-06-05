package memstore

import (
	"slices"
	"sync"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// InMemoryPathIndex is a thread-safe ports.PathIndex backed by an in-memory
// map. Suitable for v1 single-process deployments and for all tests; a
// SQLite or Postgres adapter would replace it without any consumer change.
// See docs/auth-design.md §4.2.
type InMemoryPathIndex struct {
	mu     sync.RWMutex
	byRepo map[repoOIDKey]map[string]struct{}
}

type repoOIDKey struct {
	repo string
	oid  string
}

// NewInMemoryPathIndex returns a fresh, empty index.
func NewInMemoryPathIndex() *InMemoryPathIndex {
	return &InMemoryPathIndex{
		byRepo: make(map[repoOIDKey]map[string]struct{}),
	}
}

// Record stores the (repo, oid, path) binding. Idempotent under set
// semantics: repeated calls with the same tuple are a no-op.
func (m *InMemoryPathIndex) Record(repo, oid, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := repoOIDKey{repo: repo, oid: oid}
	paths, ok := m.byRepo[key]
	if !ok {
		paths = make(map[string]struct{})
		m.byRepo[key] = paths
	}
	paths[path] = struct{}{}
	return nil
}

// PathsFor returns every path recorded for (repo, oid), sorted so that
// order is stable across calls within a process. An unknown OID returns
// a nil slice with a nil error.
func (m *InMemoryPathIndex) PathsFor(repo, oid string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	paths := m.byRepo[repoOIDKey{repo: repo, oid: oid}]
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(paths))
	for p := range paths {
		out = append(out, p)
	}
	slices.Sort(out)
	return out, nil
}

// PathsInRepo returns the deduplicated union of every path recorded under repo
// across all OIDs, sorted for stable order. An unknown repo returns a nil slice
// with a nil error. Used by the name-hiding reveal endpoint (GET /{repo}/names).
func (m *InMemoryPathIndex) PathsInRepo(repo string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{})
	for key, paths := range m.byRepo {
		if key.repo != repo {
			continue
		}
		for p := range paths {
			seen[p] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out, nil
}

var _ ports.PathIndex = (*InMemoryPathIndex)(nil)
