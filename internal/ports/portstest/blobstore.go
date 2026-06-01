package portstest

import (
	"bytes"
	"io"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// RunBlobStoreContract exercises every clause of the ports.BlobStore contract.
// Every BlobStore implementation must pass it.
//
// The factory must return a fresh, empty BlobStore on each call so subtests do
// not contaminate each other.
//
// See docs/auth-design.md §4.5.
func RunBlobStoreContract(t *testing.T, factory func() ports.BlobStore) {
	t.Helper()

	t.Run("PutThenOpenRoundTrips", func(t *testing.T) {
		bs := factory()
		want := []byte("hello blob")
		if err := bs.Put("repoA", "oid1", bytes.NewReader(want)); err != nil {
			t.Fatalf("put: %v", err)
		}
		ok, err := bs.Exists("repoA", "oid1")
		if err != nil || !ok {
			t.Fatalf("exists = (%v, %v), want (true, nil)", ok, err)
		}
		rc, err := bs.Open("repoA", "oid1")
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer rc.Close()
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("round-trip = %q, want %q", got, want)
		}
	})

	t.Run("ExistsFalseForMissing", func(t *testing.T) {
		bs := factory()
		ok, err := bs.Exists("repoA", "missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected exists=false for missing object")
		}
	})

	t.Run("OpenMissingErrors", func(t *testing.T) {
		bs := factory()
		if _, err := bs.Open("repoA", "missing"); err == nil {
			t.Error("expected error opening missing object")
		}
	})

	t.Run("PutOverwriteIsIdempotent", func(t *testing.T) {
		bs := factory()
		payload := []byte("same bytes")
		for i := 0; i < 2; i++ {
			if err := bs.Put("repoA", "oid1", bytes.NewReader(payload)); err != nil {
				t.Fatalf("put %d: %v", i, err)
			}
		}
		rc, err := bs.Open("repoA", "oid1")
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer rc.Close()
		got, _ := io.ReadAll(rc)
		if !bytes.Equal(got, payload) {
			t.Errorf("after re-put = %q, want %q", got, payload)
		}
	})

	t.Run("CrossRepoIsolation", func(t *testing.T) {
		bs := factory()
		if err := bs.Put("repoA", "oid1", bytes.NewReader([]byte("x"))); err != nil {
			t.Fatal(err)
		}
		ok, err := bs.Exists("repoB", "oid1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("repoB leaked repoA's object")
		}
	})
}
