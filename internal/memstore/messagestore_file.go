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

// FileMessageStore is a ports.MessageStore backed by an in-memory map persisted
// to a JSON file on every Put, so redacted commit messages and their bound path
// sets survive process restarts. The on-disk shape is repo -> oid -> {paths,
// message}, where message is the raw bytes (base64-encoded in JSON).
//
// Writes are whole-file and atomic (temp file + rename); suitable for the v1
// single-process server. A SQLite/Postgres adapter would replace it without any
// consumer change. See docs/auth-design.md §4.2.
type FileMessageStore struct {
	mu   sync.RWMutex
	path string
	// repo -> oid -> entry
	data map[string]map[string]messageEntry
}

type messageEntry struct {
	Paths   []string `json:"paths"`
	Message []byte   `json:"message"`
}

// NewFileMessageStore loads any existing store at path (a missing file is an
// empty store) and returns a ready-to-use, persistent store.
func NewFileMessageStore(path string) (*FileMessageStore, error) {
	m := &FileMessageStore{
		path: path,
		data: make(map[string]map[string]messageEntry),
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *FileMessageStore) load() error {
	b, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var raw map[string]map[string]messageEntry
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if raw != nil {
		m.data = raw
	}
	return nil
}

// persistLocked writes the whole store to disk atomically. Callers must hold
// the write lock.
func (m *FileMessageStore) persistLocked() error {
	b, err := json.MarshalIndent(m.data, "", "  ")
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

// Put stores the message bytes for (repo, oid) and binds them to a sorted,
// de-duplicated copy of paths, then persists. Idempotent on (repo, oid):
// re-Putting overwrites the entry.
func (m *FileMessageStore) Put(repo, oid string, paths []string, message []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sorted := slices.Clone(paths)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	if m.data[repo] == nil {
		m.data[repo] = make(map[string]messageEntry)
	}
	m.data[repo][oid] = messageEntry{Paths: sorted, Message: message}
	return m.persistLocked()
}

// BoundPaths returns a copy of the paths bound to (repo, oid). An unknown OID
// returns a nil slice with a nil error.
func (m *FileMessageStore) BoundPaths(repo, oid string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.data[repo][oid]
	if !ok || len(entry.Paths) == 0 {
		return nil, nil
	}
	return slices.Clone(entry.Paths), nil
}

// Message returns a copy of the stored bytes for (repo, oid). An unknown OID
// returns nil bytes with a nil error (the caller maps it to 404).
func (m *FileMessageStore) Message(repo, oid string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.data[repo][oid]
	if !ok {
		return nil, nil
	}
	return slices.Clone(entry.Message), nil
}

var _ ports.MessageStore = (*FileMessageStore)(nil)
