package pep_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/pep"
)

func TestParseBatchRequest(t *testing.T) {
	const body = `{
		"operation": "download",
		"transfers": ["basic"],
		"ref": {"name": "refs/heads/main"},
		"hash_algo": "sha256",
		"objects": [
			{"oid": "abc123", "name": "mine/x.bin", "size": 12},
			{"oid": "def456", "name": "src/y.bin", "size": 34}
		]
	}`
	req := httptest.NewRequest("POST", "/objects/batch", strings.NewReader(body))
	got, err := pep.ParseBatchRequest(req)
	if err != nil {
		t.Fatalf("ParseBatchRequest: %v", err)
	}
	if got.Operation != pep.OpDownload {
		t.Errorf("Operation = %q, want %q", got.Operation, pep.OpDownload)
	}
	if len(got.Objects) != 2 {
		t.Fatalf("len(Objects) = %d, want 2", len(got.Objects))
	}
	if got.Objects[0].OID != "abc123" || got.Objects[0].Name != "mine/x.bin" || got.Objects[0].Size != 12 {
		t.Errorf("Objects[0] = %+v", got.Objects[0])
	}
}

func TestParseBatchRequestRejectsBad(t *testing.T) {
	cases := []struct{ name, body string }{
		{"malformed json", `{ not json`},
		{"unknown operation", `{"operation":"delete","objects":[{"oid":"a"}]}`},
		{"missing operation", `{"objects":[{"oid":"a"}]}`},
		{"empty objects", `{"operation":"download","objects":[]}`},
		{"object missing oid", `{"operation":"download","objects":[{"name":"x"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/objects/batch", strings.NewReader(tc.body))
			if _, err := pep.ParseBatchRequest(req); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// The real client always sends transfers/ref/hash_algo and may send name as
// omitempty. The adapter must tolerate the optional fields and an absent name.
func TestParseBatchRequestToleratesOptionalFields(t *testing.T) {
	const body = `{
		"operation": "upload",
		"transfers": ["basic", "lfs-standalone-file"],
		"ref": {"name": "refs/heads/main"},
		"hash_algo": "sha256",
		"objects": [{"oid": "abc", "size": 1}]
	}`
	req := httptest.NewRequest("POST", "/objects/batch", strings.NewReader(body))
	got, err := pep.ParseBatchRequest(req)
	if err != nil {
		t.Fatalf("should tolerate optional fields: %v", err)
	}
	if got.Objects[0].Name != "" {
		t.Errorf("absent name should be empty, got %q", got.Objects[0].Name)
	}
}
