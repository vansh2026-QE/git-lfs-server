package memstore

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// LocalFSObjectStore is the demo storage backend. It satisfies both
// ports.ObjectStore (minting same-origin transfer URLs) and ports.BlobStore
// (storing the bytes on local disk under {root}/{repo}/{oid}). Production
// splits these: an S3ObjectStore mints pre-signed URLs and there is no
// BlobStore. See docs/auth-design.md §4.5.
type LocalFSObjectStore struct {
	root    string
	baseURL string
}

// NewLocalFSObjectStore stores bytes under root and mints hrefs rooted at
// baseURL (e.g. "http://localhost:8080"). A trailing slash on baseURL is
// trimmed so href construction is unambiguous.
func NewLocalFSObjectStore(root, baseURL string) *LocalFSObjectStore {
	return &LocalFSObjectStore{root: root, baseURL: strings.TrimRight(baseURL, "/")}
}

// MintUpload and MintDownload return the same open capability URL; path and
// size are unused by the local backend (bytes are addressed by oid).
func (s *LocalFSObjectStore) MintUpload(repo, oid, _ string, _ int64) (ports.ObjectAction, error) {
	return ports.ObjectAction{Href: s.href(repo, oid)}, nil
}

func (s *LocalFSObjectStore) MintDownload(repo, oid, _ string) (ports.ObjectAction, error) {
	return ports.ObjectAction{Href: s.href(repo, oid)}, nil
}

// href is the same-origin capability URL the open transfer endpoints serve.
func (s *LocalFSObjectStore) href(repo, oid string) string {
	return s.baseURL + "/" + path.Join(repo, "objects", oid)
}

func (s *LocalFSObjectStore) Put(repo, oid string, r io.Reader) error {
	dir, full, err := s.objectPath(repo, oid)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Write to a temp file then rename so a concurrent Open never observes a
	// partially written object.
	tmp, err := os.CreateTemp(dir, "."+oid+".tmp-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), full)
}

func (s *LocalFSObjectStore) Open(repo, oid string) (io.ReadCloser, error) {
	_, full, err := s.objectPath(repo, oid)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (s *LocalFSObjectStore) Exists(repo, oid string) (bool, error) {
	_, full, err := s.objectPath(repo, oid)
	if err != nil {
		return false, err
	}
	switch _, statErr := os.Stat(full); {
	case statErr == nil:
		return true, nil
	case os.IsNotExist(statErr):
		return false, nil
	default:
		return false, statErr
	}
}

// objectPath validates the segments and returns the containing dir and full
// file path. repo and oid must each be a single safe segment so a crafted
// value cannot escape root via separators or "..".
func (s *LocalFSObjectStore) objectPath(repo, oid string) (dir, full string, err error) {
	if !safeSegment(repo) || !safeSegment(oid) {
		return "", "", fmt.Errorf("memstore: unsafe repo/oid segment %q/%q", repo, oid)
	}
	dir = filepath.Join(s.root, repo)
	return dir, filepath.Join(dir, oid), nil
}

func safeSegment(s string) bool {
	return s != "" && s != "." && s != ".." &&
		!strings.ContainsAny(s, `/\`) && !strings.Contains(s, "..")
}

var (
	_ ports.ObjectStore = (*LocalFSObjectStore)(nil)
	_ ports.BlobStore   = (*LocalFSObjectStore)(nil)
)
