package pep_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/pep"
)

func TestVerifyDownloadClaim(t *testing.T) {
	idx := memstore.NewInMemoryPathIndex()
	_ = idx.Record("repo", "oid1", "a/1.bin")
	_ = idx.Record("repo", "oid2", "x/1.bin")
	_ = idx.Record("repo", "oid2", "y/2.bin")

	cases := []struct {
		name       string
		oid, claim string
		wantPath   string
		wantStatus int
		wantErr    error
	}{
		{"matching claim", "oid1", "a/1.bin", "a/1.bin", http.StatusOK, nil},
		{"empty claim uses first recorded", "oid1", "", "a/1.bin", http.StatusOK, nil},
		{"empty claim multi-path deterministic", "oid2", "", "x/1.bin", http.StatusOK, nil},
		{"wrong claim rejected", "oid1", "evil/secret.bin", "", http.StatusForbidden, pep.ErrNameMismatch},
		{"unknown oid", "ghost", "a/1.bin", "", http.StatusNotFound, pep.ErrUnknownOID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, status, err := pep.VerifyDownloadClaim(idx, "repo", tc.oid, tc.claim)
			if path != tc.wantPath || status != tc.wantStatus || !errors.Is(err, tc.wantErr) {
				t.Errorf("got (%q, %d, %v), want (%q, %d, %v)",
					path, status, err, tc.wantPath, tc.wantStatus, tc.wantErr)
			}
		})
	}
}

func TestVerifyDownloadClaimCrossRepoIsolation(t *testing.T) {
	idx := memstore.NewInMemoryPathIndex()
	_ = idx.Record("repoA", "oid1", "a/1.bin")
	if _, status, err := pep.VerifyDownloadClaim(idx, "repoB", "oid1", "a/1.bin"); status != http.StatusNotFound || !errors.Is(err, pep.ErrUnknownOID) {
		t.Errorf("cross-repo lookup should 404, got status %d err %v", status, err)
	}
}

// errIndex makes PathsFor fail so the 500 path is covered.
type errIndex struct{}

func (errIndex) Record(string, string, string) error       { return nil }
func (errIndex) PathsFor(string, string) ([]string, error) { return nil, errors.New("boom") }

func TestVerifyDownloadClaimInfraError(t *testing.T) {
	if _, status, err := pep.VerifyDownloadClaim(errIndex{}, "repo", "oid", ""); status != http.StatusInternalServerError || err == nil {
		t.Errorf("infra error should 500, got status %d err %v", status, err)
	}
}

func TestVerifyUploadClaim(t *testing.T) {
	if path, status, err := pep.VerifyUploadClaim("mine/x.bin"); path != "mine/x.bin" || status != http.StatusOK || err != nil {
		t.Errorf("valid upload claim: got (%q, %d, %v)", path, status, err)
	}
	if _, status, err := pep.VerifyUploadClaim(""); status != http.StatusBadRequest || !errors.Is(err, pep.ErrEmptyName) {
		t.Errorf("empty upload name should 400, got status %d err %v", status, err)
	}
}
