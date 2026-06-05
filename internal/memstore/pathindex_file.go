package memstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// FilePathIndex is a ports.PathIndex backed by an in-memory map that is
// persisted to a JSON file on every Record, so (repo, oid) -> paths bindings
// survive process restarts. The on-disk shape is repo -> oid -> sorted paths.
//
// Writes are whole-file and atomic (temp file + rename); suitable for the v1
// single-process server. A SQLite/Postgres adapter would replace it without
// any consumer change. See docs/auth-design.md §4.2.
type FilePathIndex struct {
	mu   sync.RWMutex
	path string
	// repo -> oid -> set of paths
	data map[string]map[string]map[string]struct{}
}

// NewFilePathIndex loads any existing index at path (a missing file is an empty
// index) and returns a ready-to-use, persistent index.
func NewFilePathIndex(path string) (*FilePathIndex, error) {
	m := &FilePathIndex{
		path: path,
		data: make(map[string]map[string]map[string]struct{}),
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *FilePathIndex) load() error {
	b, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw map[string]map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	for repo, oids := range raw {
		rm := make(map[string]map[string]struct{}, len(oids))
		for oid, paths := range oids {
			set := make(map[string]struct{}, len(paths))
			for _, p := range paths {
				set[p] = struct{}{}
			}
			rm[oid] = set
		}
		m.data[repo] = rm
	}
	return nil
}

// persistLocked writes the whole index to disk atomically. Callers must hold
// the write lock.
func (m *FilePathIndex) persistLocked() error {
	raw := make(map[string]map[string][]string, len(m.data))
	for repo, oids := range m.data {
		om := make(map[string][]string, len(oids))
		for oid, set := range oids {
			paths := make([]string, 0, len(set))
			for p := range set {
				paths = append(paths, p)
			}
			slices.Sort(paths)
			om[oid] = paths
		}
		raw[repo] = om
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// Record stores the (repo, oid, path) binding and persists. Idempotent under
// set semantics: repeated calls with the same tuple change nothing on disk.
func (m *FilePathIndex) Record(repo, oid, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[repo] == nil {
		m.data[repo] = make(map[string]map[string]struct{})
	}
	if m.data[repo][oid] == nil {
		m.data[repo][oid] = make(map[string]struct{})
	}
	if _, ok := m.data[repo][oid][path]; ok {
		return nil // already recorded; avoid a redundant disk write
	}
	m.data[repo][oid][path] = struct{}{}
	return m.persistLocked()
}

// PathsFor returns every path recorded for (repo, oid), sorted for stable
// order. An unknown OID returns a nil slice with a nil error.
func (m *FilePathIndex) PathsFor(repo, oid string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set := m.data[repo][oid]
	if len(set) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	slices.Sort(out)
	return out, nil
}

// PathsInRepo returns the deduplicated union of every path recorded under repo
// across all OIDs, sorted for stable order. An unknown repo returns a nil slice
// with a nil error. Read-only: it does not persist. Used by the name-hiding
// reveal endpoint (GET /{repo}/names).
func (m *FilePathIndex) PathsInRepo(repo string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{})
	for _, set := range m.data[repo] {
		for p := range set {
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

var _ ports.PathIndex = (*FilePathIndex)(nil)
