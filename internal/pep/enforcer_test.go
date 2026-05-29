package pep

import (
	"errors"
	"net/http"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// fakeStore mints deterministic hrefs, or fails when an error is configured.
type fakeStore struct {
	uploadErr, downloadErr error
}

func (f fakeStore) MintUpload(repo, oid, path string, size int64) (ports.ObjectAction, error) {
	if f.uploadErr != nil {
		return ports.ObjectAction{}, f.uploadErr
	}
	return ports.ObjectAction{Href: "https://store/upload/" + oid}, nil
}

func (f fakeStore) MintDownload(repo, oid, path string) (ports.ObjectAction, error) {
	if f.downloadErr != nil {
		return ports.ObjectAction{}, f.downloadErr
	}
	return ports.ObjectAction{Href: "https://store/download/" + oid}, nil
}

func permit() policy.Decision { return policy.Decision{Effect: policy.Permit} }
func deny() policy.Decision {
	return policy.Decision{Effect: policy.Deny, Reason: "no grant matched any principal"}
}

func TestEnforceDownload(t *testing.T) {
	outcomes := []objectOutcome{
		{OID: "ok", Size: 1, Path: "pub/a.bin", Decision: permit()},
		{OID: "denied", Size: 2, Path: "secret/b.bin", Decision: deny()},
		{OID: "missing", Size: 3, Status: http.StatusNotFound, VerifyErr: ErrUnknownOID},
	}

	resp, err := EnforceDownload(fakeStore{}, "r", outcomes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Objects) != 3 {
		t.Fatalf("objects = %d, want 3", len(resp.Objects))
	}

	if a := resp.Objects[0].Actions["download"]; a == nil || a.Href != "https://store/download/ok" {
		t.Errorf("permit object missing download action: %+v", resp.Objects[0])
	}
	if resp.Objects[0].Error != nil {
		t.Errorf("permit object should have no error: %+v", resp.Objects[0].Error)
	}

	if e := resp.Objects[1].Error; e == nil || e.Code != http.StatusForbidden {
		t.Errorf("denied object = %+v, want 403 error", resp.Objects[1])
	}
	if resp.Objects[1].Actions != nil {
		t.Errorf("denied object should mint no action")
	}

	if e := resp.Objects[2].Error; e == nil || e.Code != http.StatusNotFound {
		t.Errorf("missing object = %+v, want 404 error", resp.Objects[2])
	}
}

func TestEnforceDownloadMintErrorAbortsRequest(t *testing.T) {
	outcomes := []objectOutcome{{OID: "ok", Path: "pub/a.bin", Decision: permit()}}
	sentinel := errors.New("storage down")

	_, err := EnforceDownload(fakeStore{downloadErr: sentinel}, "r", outcomes)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wrapped %v", err, sentinel)
	}
}

func TestEnforceUploadAllPermit(t *testing.T) {
	outcomes := []objectOutcome{
		{OID: "a", Size: 1, Path: "mine/x.bin", Decision: permit()},
		{OID: "b", Size: 2, Path: "mine/y.bin", Decision: permit()},
	}

	resp, rej, err := EnforceUpload(fakeStore{}, "r", outcomes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rej != nil {
		t.Fatalf("unexpected rejection: %+v", rej)
	}
	if len(resp.Objects) != 2 {
		t.Fatalf("objects = %d, want 2", len(resp.Objects))
	}
	if a := resp.Objects[0].Actions["upload"]; a == nil || a.Href != "https://store/upload/a" {
		t.Errorf("object a missing upload action: %+v", resp.Objects[0])
	}
}

func TestEnforceUploadOneDenyRejectsBatch(t *testing.T) {
	outcomes := []objectOutcome{
		{OID: "a", Path: "mine/x.bin", Decision: permit()},
		{OID: "b", Path: "secret/db.bin", Decision: deny()},
	}

	resp, rej, err := EnforceUpload(fakeStore{}, "r", outcomes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Fatalf("rejected batch should mint no response: %+v", resp)
	}
	if rej == nil {
		t.Fatal("expected rejection, got nil")
	}
	if rej.Repo != "r" || len(rej.Paths) != 1 || rej.Paths[0] != "secret/db.bin" {
		t.Errorf("rejection = %+v, want repo r with [secret/db.bin]", rej)
	}
}

func TestEnforceUploadMintErrorAbortsRequest(t *testing.T) {
	outcomes := []objectOutcome{{OID: "a", Path: "mine/x.bin", Decision: permit()}}
	sentinel := errors.New("storage down")

	_, _, err := EnforceUpload(fakeStore{uploadErr: sentinel}, "r", outcomes)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wrapped %v", err, sentinel)
	}
}
