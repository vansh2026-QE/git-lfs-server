package memstore_test

import (
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/ports"
	"github.com/vansh2026/git-lfs-server/internal/ports/portstest"
)

func TestInMemoryPathIndex_Contract(t *testing.T) {
	portstest.RunPathIndexContract(t, func() ports.PathIndex {
		return memstore.NewInMemoryPathIndex()
	})
}
