package pep_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/loader"
	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/pep"
)

const demoPolicy = `{
  "version": 1,
  "repos": {
    "demo": {
      "paths": {
        "public/**":            { "user:alice": "rw", "user:bob": "rw" },
        "private/privAlice/**": { "user:alice": "rw" },
        "private/privBob/**":   { "user:bob": "rw" }
      }
    }
  }
}`

func newDemoServer(t *testing.T) *pep.Server {
	t.Helper()
	return newDemoLocalServer(t).Server
}

// newDemoLocalServer builds the composed demo handler: a batch Server plus the
// open transfer endpoints, sharing one LocalFSObjectStore as both the
// ObjectStore (mint) and the BlobStore (bytes).
func newDemoLocalServer(t *testing.T) *pep.LocalServer {
	t.Helper()
	res, err := loader.New(memstore.NewStringPolicyStore(demoPolicy)).Load(context.Background())
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	auth := memstore.NewInMemoryAuthenticator(map[string]memstore.UserRecord{
		"alice": {Password: "pw"},
		"bob":   {Password: "pw"},
	})
	store := memstore.NewLocalFSObjectStore(t.TempDir(), "http://example.test")
	srv := pep.NewServer(auth, memstore.NewInMemoryPathIndex(), store, memstore.NewStderrAuditSink(), res.Policy)
	return pep.NewLocalServer(srv, store)
}

// doBatch posts a batch request as user (empty user = anonymous) and returns
// the recorder plus the decoded response.
func doBatch(t *testing.T, srv http.Handler, user, op, body string) (*httptest.ResponseRecorder, pep.BatchResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/demo/objects/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	if user != "" {
		req.SetBasicAuth(user, "pw")
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var decoded pep.BatchResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
		}
	}
	return rec, decoded
}

func uploadBody(oid, name string) string {
	return `{"operation":"upload","objects":[{"oid":"` + oid + `","name":"` + name + `","size":4}]}`
}

func downloadBody(oid, name string) string {
	return `{"operation":"download","objects":[{"oid":"` + oid + `","name":"` + name + `","size":4}]}`
}

func TestBatchUploadPermitMintsAction(t *testing.T) {
	srv := newDemoServer(t)
	rec, resp := doBatch(t, srv, "alice", "upload", uploadBody("oidpub", "public/x.bin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	o := resp.Objects[0]
	if o.Actions["upload"] == nil {
		t.Fatalf("expected upload action, got %+v", o)
	}
	if !o.Authenticated {
		t.Errorf("permitted object should be authenticated")
	}
}

func TestBatchUploadDenyRejectsWholeBatch(t *testing.T) {
	srv := newDemoServer(t)
	// bob may not upload under privAlice -> all-or-nothing 403.
	rec, _ := doBatch(t, srv, "bob", "upload", uploadBody("oidx", "private/privAlice/secret.bin"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
}

func TestBatchDownloadPermitAfterUpload(t *testing.T) {
	srv := newDemoServer(t)
	// alice uploads public/x.bin -> records the binding at batch time.
	if rec, _ := doBatch(t, srv, "alice", "upload", uploadBody("oidpub", "public/x.bin")); rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d", rec.Code)
	}
	// bob downloads it (public is readable by bob) -> per-object download action.
	rec, resp := doBatch(t, srv, "bob", "download", downloadBody("oidpub", "public/x.bin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if resp.Objects[0].Actions["download"] == nil {
		t.Fatalf("expected download action, got %+v", resp.Objects[0])
	}
}

func TestBatchDownloadDeniesPrivateOfOtherUser(t *testing.T) {
	srv := newDemoServer(t)
	// alice uploads a private object under her own prefix.
	if rec, _ := doBatch(t, srv, "alice", "upload", uploadBody("oidpriv", "private/privAlice/s.bin")); rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d", rec.Code)
	}
	// bob tries to download it: claim matches the recorded path, but the PDP
	// denies -> per-object 403 inside a 200 batch.
	rec, resp := doBatch(t, srv, "bob", "download", downloadBody("oidpriv", "private/privAlice/s.bin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	e := resp.Objects[0].Error
	if e == nil || e.Code != http.StatusForbidden {
		t.Fatalf("expected per-object 403, got %+v", resp.Objects[0])
	}
	if resp.Objects[0].Actions != nil {
		t.Errorf("denied object must carry no action")
	}
}

func TestBatchInvalidCredentialsUnauthorized(t *testing.T) {
	srv := newDemoServer(t)
	req := httptest.NewRequest(http.MethodPost, "/demo/objects/batch", strings.NewReader(uploadBody("o", "public/x")))
	req.SetBasicAuth("alice", "wrong")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestBatchMalformedBadRequest(t *testing.T) {
	srv := newDemoServer(t)
	rec, _ := doBatch(t, srv, "alice", "upload", `{ not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestObjectRoundTripThroughTransferEndpoints(t *testing.T) {
	srv := newDemoLocalServer(t)
	const oid, content = "oidpub", "data"

	// Authorize + record via the batch endpoint, and capture the minted href.
	rec, resp := doBatch(t, srv, "alice", "upload", uploadBody(oid, "public/x.bin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload batch status = %d\n%s", rec.Code, rec.Body.String())
	}
	target := mustPath(t, resp.Objects[0].Actions["upload"].Href)

	// PUT the bytes to the minted capability URL (transfer carries no auth).
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, httptest.NewRequest(http.MethodPut, target, strings.NewReader(content)))
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", putRec.Code)
	}

	// GET them back and confirm the bytes round-trip.
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, target, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", getRec.Code)
	}
	if got := getRec.Body.String(); got != content {
		t.Errorf("round-trip body = %q, want %q", got, content)
	}
}

func TestGetUnknownObjectNotFound(t *testing.T) {
	srv := newDemoLocalServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/demo/objects/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func mustPath(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse href %q: %v", rawURL, err)
	}
	return u.Path
}
