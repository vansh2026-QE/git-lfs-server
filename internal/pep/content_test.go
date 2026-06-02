package pep_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// putBlob streams body into the blob store via the open PUT transfer endpoint.
func putBlob(t *testing.T, srv http.Handler, oid, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/demo/objects/"+oid, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put blob status = %d", rec.Code)
	}
}

// getContent calls the authenticated content endpoint as user (empty = no creds).
func getContent(t *testing.T, srv http.Handler, user, oid, name string) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{}
	if oid != "" {
		q.Set("oid", oid)
	}
	if name != "" {
		q.Set("name", name)
	}
	req := httptest.NewRequest(http.MethodGet, "/demo/content?"+q.Encode(), nil)
	if user != "" {
		req.SetBasicAuth(user, "pw")
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestContentPermitStreamsBytes(t *testing.T) {
	ls := newDemoLocalServer(t)
	// alice uploads public/x.bin -> records the (oid, path) binding.
	if rec, _ := doBatch(t, ls, "alice", "upload", uploadBody("oidpub", "public/x.bin")); rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d", rec.Code)
	}
	putBlob(t, ls, "oidpub", "DATA")

	// bob may read public/** -> bytes stream back.
	rec := getContent(t, ls, "bob", "oidpub", "public/x.bin")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "DATA" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "DATA")
	}
}

func TestContentDeniedPathForbidden(t *testing.T) {
	ls := newDemoLocalServer(t)
	if rec, _ := doBatch(t, ls, "alice", "upload", uploadBody("oidpriv", "private/privAlice/s.bin")); rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d", rec.Code)
	}
	putBlob(t, ls, "oidpriv", "SECRET")

	// bob may not read alice's private prefix -> 403, no bytes.
	rec := getContent(t, ls, "bob", "oidpriv", "private/privAlice/s.bin")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403\n%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET") {
		t.Error("denied response leaked object bytes")
	}
}

func TestContentUnknownOIDNotFound(t *testing.T) {
	ls := newDemoLocalServer(t)
	rec := getContent(t, ls, "bob", "neverrecorded", "public/x.bin")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\n%s", rec.Code, rec.Body.String())
	}
}

func TestContentMissingOIDBadRequest(t *testing.T) {
	ls := newDemoLocalServer(t)
	rec := getContent(t, ls, "bob", "", "public/x.bin")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
}

func TestContentInvalidCredentialsUnauthorized(t *testing.T) {
	ls := newDemoLocalServer(t)
	req := httptest.NewRequest(http.MethodGet, "/demo/content?oid=o&name=public/x.bin", nil)
	req.SetBasicAuth("alice", "wrong")
	rec := httptest.NewRecorder()
	ls.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestContentRequireAuthRejectsAnonymous(t *testing.T) {
	ls := newDemoLocalServer(t)
	ls.SetRequireAuth(true)
	rec := getContent(t, ls, "", "oidpub", "public/x.bin")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401\n%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate challenge under requireAuth")
	}
}
