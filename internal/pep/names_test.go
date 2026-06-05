package pep_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// doNames issues GET /demo/names as user (empty user = anonymous) against srv
// and returns the recorder plus the decoded path list.
func doNames(t *testing.T, srv http.Handler, user string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/demo/names", nil)
	if user != "" {
		req.SetBasicAuth(user, "pw")
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var decoded struct {
		Paths []string `json:"paths"`
	}
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
		}
	}
	return rec, decoded.Paths
}

// seedDemoUploads records one object per path via the batch endpoint, the same
// way a real upload populates the PathIndex.
func seedDemoUploads(t *testing.T, srv http.Handler) {
	t.Helper()
	uploads := []struct{ user, oid, name string }{
		{"alice", "oidpub", "public/x.bin"},
		{"alice", "oida", "private/privAlice/a.bin"},
		{"bob", "oidb", "private/privBob/b.bin"},
	}
	for _, u := range uploads {
		if rec, _ := doBatch(t, srv, u.user, "upload", uploadBody(u.oid, u.name)); rec.Code != http.StatusOK {
			t.Fatalf("seed upload %q: status %d\n%s", u.name, rec.Code, rec.Body.String())
		}
	}
}

func TestNamesReturnsAuthorizedSubset(t *testing.T) {
	srv := newDemoLocalServer(t)
	seedDemoUploads(t, srv)

	rec, paths := doNames(t, srv, "bob")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	slices.Sort(paths)
	want := []string{"private/privBob/b.bin", "public/x.bin"}
	if !slices.Equal(paths, want) {
		t.Errorf("bob names = %v, want %v", paths, want)
	}
}

func TestNamesExcludesDeniedPaths(t *testing.T) {
	srv := newDemoLocalServer(t)
	seedDemoUploads(t, srv)

	_, paths := doNames(t, srv, "bob")
	if slices.Contains(paths, "private/privAlice/a.bin") {
		t.Errorf("bob must not see Alice's private path: %v", paths)
	}

	_, alicePaths := doNames(t, srv, "alice")
	slices.Sort(alicePaths)
	want := []string{"private/privAlice/a.bin", "public/x.bin"}
	if !slices.Equal(alicePaths, want) {
		t.Errorf("alice names = %v, want %v", alicePaths, want)
	}
}

func TestNamesAnonymousSeesNothing(t *testing.T) {
	srv := newDemoLocalServer(t)
	seedDemoUploads(t, srv)

	// The demo backend leaves auth optional, so anonymous is run through the
	// policy and (having no grants) reveals nothing rather than being rejected.
	rec, paths := doNames(t, srv, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if len(paths) != 0 {
		t.Errorf("anonymous should see no names, got %v", paths)
	}
}

func TestNamesEmptyRepo(t *testing.T) {
	srv := newDemoLocalServer(t)
	rec, paths := doNames(t, srv, "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if len(paths) != 0 {
		t.Errorf("empty repo should yield no names, got %v", paths)
	}
}
