package memstore

import (
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// StderrAuditSink is a ports.AuditSink that writes one JSON line per
// AuditEntry to a configurable io.Writer (os.Stderr by default).
//
// Writes are synchronous and serialised by an internal mutex: Record
// returns only after the line is flushed, and concurrent calls cannot
// interleave bytes. For stderr this is fast enough; networked production
// sinks (OTel, syslog) should buffer through a channel and a background
// flush goroutine instead. See docs/auth-design.md §4.4.
type StderrAuditSink struct {
	mu  sync.Mutex
	out io.Writer
}

// NewStderrAuditSink returns a sink that writes to os.Stderr. Use
// NewStderrAuditSinkTo when tests or deployments need to redirect output.
func NewStderrAuditSink() *StderrAuditSink {
	return &StderrAuditSink{out: os.Stderr}
}

// NewStderrAuditSinkTo returns a sink that writes to out. Used by the
// contract suite (which passes a *bytes.Buffer) and by deployments that
// prefer stdout or a log file.
func NewStderrAuditSinkTo(out io.Writer) *StderrAuditSink {
	return &StderrAuditSink{out: out}
}

// Record marshals the entry as JSON, appends '\n', and writes the line.
// Errors from json.Marshal or the underlying writer are swallowed: the
// audit sink must never propagate failures back to the PEP.
func (s *StderrAuditSink) Record(entry ports.AuditEntry) {
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line = append(line, '\n')
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.out.Write(line)
}

var _ ports.AuditSink = (*StderrAuditSink)(nil) // static assertion, that StderrAuditSink implements the AuditSink interface
