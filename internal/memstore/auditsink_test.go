package memstore_test

import (
	"io"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/ports"
	"github.com/vansh2026/git-lfs-server/internal/ports/portstest"
)

func TestStderrAuditSink_Contract(t *testing.T) {
	portstest.RunAuditSinkContract(t, func(out io.Writer) ports.AuditSink {
		return memstore.NewStderrAuditSinkTo(out)
	})
}
