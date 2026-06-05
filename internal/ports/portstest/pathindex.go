package portstest

import (
	"slices"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// RunPathIndexContract exercises every clause of the ports.PathIndex
// contract. Every PathIndex implementation must pass it.
//
// The factory must return a fresh, empty PathIndex on each call so subtests
// do not contaminate each other.
//
// See docs/auth-design.md §4.2 and §9.
func RunPathIndexContract(t *testing.T, factory func() ports.PathIndex) {
	t.Helper()

	t.Run("UnknownOIDReturnsEmpty", func(t *testing.T) {
		idx := factory()
		paths, err := idx.PathsFor("repoA", "oid-unknown")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("expected empty slice, got %v", paths)
		}
	})

	t.Run("RecordIsIdempotent", func(t *testing.T) {
		idx := factory()
		for i := 0; i < 3; i++ {
			if err := idx.Record("repoA", "oid1", "p/x"); err != nil {
				t.Fatalf("record %d: %v", i, err)
			}
		}
		paths, err := idx.PathsFor("repoA", "oid1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(paths) != 1 || paths[0] != "p/x" {
			t.Errorf("expected [p/x], got %v", paths)
		}
	})

	t.Run("MultiplePathsForOneOID", func(t *testing.T) {
		idx := factory()
		want := []string{"a/1", "a/2", "b/3"}
		for _, p := range want {
			if err := idx.Record("repoA", "oid1", p); err != nil {
				t.Fatalf("record %s: %v", p, err)
			}
		}
		got, err := idx.PathsFor("repoA", "oid1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("CrossRepoIsolation", func(t *testing.T) {
		idx := factory()
		if err := idx.Record("repoA", "oid1", "p/x"); err != nil {
			t.Fatal(err)
		}
		paths, err := idx.PathsFor("repoB", "oid1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("repoB leaked repoA's binding: %v", paths)
		}
	})

	t.Run("PathsInRepoDedupAcrossOIDs", func(t *testing.T) {
		idx := factory()
		// Same path under two OIDs plus a distinct path; the union dedups.
		for _, rec := range []struct{ oid, path string }{
			{"oid1", "a/1"}, {"oid2", "a/1"}, {"oid2", "b/2"},
		} {
			if err := idx.Record("repoA", rec.oid, rec.path); err != nil {
				t.Fatalf("record %+v: %v", rec, err)
			}
		}
		got, err := idx.PathsInRepo("repoA")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"a/1", "b/2"}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("PathsInRepoUnknownRepoEmpty", func(t *testing.T) {
		idx := factory()
		paths, err := idx.PathsInRepo("repo-unknown")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("expected empty slice, got %v", paths)
		}
	})

	t.Run("PathsInRepoCrossRepoIsolation", func(t *testing.T) {
		idx := factory()
		if err := idx.Record("repoA", "oid1", "a/1"); err != nil {
			t.Fatal(err)
		}
		paths, err := idx.PathsInRepo("repoB")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("repoB leaked repoA's paths: %v", paths)
		}
	})
}
