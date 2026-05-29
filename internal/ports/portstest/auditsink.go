package portstest

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// RunAuditSinkContract exercises every clause of the ports.AuditSink
// contract. Every AuditSink implementation must pass it.
//
// The factory returns a fresh sink writing to the supplied io.Writer; the
// suite uses a bytes.Buffer so it can read what the sink emitted.
//
// See docs/auth-design.md §4.4.
func RunAuditSinkContract(t *testing.T, factory func(out io.Writer) ports.AuditSink) {
	t.Helper()

	sample := ports.AuditEntry{
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		RequestID: "req-1",
		Subject:   "user:alice",
		Action:    "download",
		Repo:      "myrepo",
		Path:      "mine/x.bin",
		OID:       "abc123",
		Effect:    "permit",
		Source:    "user:alice",
		Reason:    "permitted by user:alice",
	}

	t.Run("RecordEmitsOneJSONLine", func(t *testing.T) {
		var buf bytes.Buffer
		sink := factory(&buf)
		sink.Record(sample)

		out := buf.String()
		if !strings.HasSuffix(out, "\n") {
			t.Fatalf("expected trailing newline, got %q", out)
		}
		if c := strings.Count(out, "\n"); c != 1 {
			t.Fatalf("expected exactly one line, got %d:\n%s", c, out)
		}

		var got ports.AuditEntry
		if err := json.Unmarshal([]byte(strings.TrimRight(out, "\n")), &got); err != nil {
			t.Fatalf("output not valid JSON: %v\n%s", err, out)
		}
		if !got.Timestamp.Equal(sample.Timestamp) || got.Subject != sample.Subject ||
			got.Effect != sample.Effect || got.Source != sample.Source ||
			got.Reason != sample.Reason {
			t.Errorf("decoded entry differs from sample\n got  %+v\n want %+v", got, sample)
		}
	})

	t.Run("MultipleRecordsEmitMultipleLines", func(t *testing.T) {
		var buf bytes.Buffer
		sink := factory(&buf)
		for i := 0; i < 5; i++ {
			sink.Record(sample)
		}
		if c := strings.Count(buf.String(), "\n"); c != 5 {
			t.Errorf("expected 5 lines, got %d", c)
		}
	})

	t.Run("ConcurrentRecordsDoNotInterleave", func(t *testing.T) {
		var buf bytes.Buffer
		sink := factory(&buf)
		const N = 20
		var wg sync.WaitGroup
		for i := 0; i < N; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sink.Record(sample)
			}()
		}
		wg.Wait()

		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != N {
			t.Fatalf("expected %d lines, got %d", N, len(lines))
		}
		for i, line := range lines {
			var e ports.AuditEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				t.Errorf("line %d not valid JSON (likely interleaved): %v\n%s", i, err, line)
			}
		}
	})
}
