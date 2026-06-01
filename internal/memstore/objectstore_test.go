package memstore_test

import (
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/ports"
	"github.com/vansh2026/git-lfs-server/internal/ports/portstest"
)

func TestLocalFSObjectStore_BlobContract(t *testing.T) {
	portstest.RunBlobStoreContract(t, func() ports.BlobStore {
		return memstore.NewLocalFSObjectStore(t.TempDir(), "http://localhost:8080")
	})
}

func TestLocalFSObjectStore_MintsSameOriginHrefs(t *testing.T) {
	// Trailing slash on baseURL must be trimmed so the href has no "//".
	store := memstore.NewLocalFSObjectStore(t.TempDir(), "http://localhost:8080/")

	up, err := store.MintUpload("myrepo", "abc123", "pub/a.bin", 42)
	if err != nil {
		t.Fatalf("mint upload: %v", err)
	}
	const want = "http://localhost:8080/myrepo/objects/abc123"
	if up.Href != want {
		t.Errorf("upload href = %q, want %q", up.Href, want)
	}

	down, err := store.MintDownload("myrepo", "abc123", "pub/a.bin")
	if err != nil {
		t.Fatalf("mint download: %v", err)
	}
	if down.Href != want {
		t.Errorf("download href = %q, want %q", down.Href, want)
	}
}
