package loader_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/loader"
	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/policy"
)

func TestLoadValidGoldenFiles(t *testing.T) {
	walkGolden(t, "testdata/valid", func(t *testing.T, path string) {
		t.Helper()
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ldr := loader.New(memstore.NewStringPolicyStore(string(bytes)))
		res, err := ldr.Load(context.Background())
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if res.Policy == nil || len(res.Policy.Repos) == 0 {
			t.Fatal("expected non-empty Policy")
		}
	})
}

func TestLoadInvalidGoldenFiles(t *testing.T) {
	walkGolden(t, "testdata/invalid", func(t *testing.T, path string) {
		t.Helper()
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ldr := loader.New(memstore.NewStringPolicyStore(string(bytes)))
		if _, err := ldr.Load(context.Background()); err == nil {
			t.Fatalf("expected Load error for %s", filepath.Base(path))
		}
	})
}

func TestLoadSubsumptionWarning(t *testing.T) {
	const doc = `{
  "version": 1,
  "repos": {
    "myrepo": {
      "paths": {
        "public/**":        { "user:alice": "r" },
        "public/drafts/**": { "user:alice": "r" }
      }
    }
  }
}`
	ldr := loader.New(memstore.NewStringPolicyStore(doc))
	res, err := ldr.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected subsumption warning")
	}
}

func TestLoadUndefinedGroupMembershipWarning(t *testing.T) {
	const doc = `{
  "version": 1,
  "memberships": {
    "user:alice": ["group:ghost"]
  },
  "repos": {
    "myrepo": {
      "paths": {
        "**": { "user:alice": "r" }
      }
    }
  }
}`
	ldr := loader.New(memstore.NewStringPolicyStore(doc))
	res, err := ldr.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w.Message, "group:ghost") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning about undefined group, got %v", res.Warnings)
	}
}

func TestLoadGrantIDDerivation(t *testing.T) {
	const doc = `{
  "version": 1,
  "repos": {
    "myrepo": {
      "paths": {
        "mine/**": { "user:alice": "r" }
      }
    }
  }
}`
	ldr := loader.New(memstore.NewStringPolicyStore(doc))
	res, err := ldr.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := policy.GrantID("myrepo|user:alice|download|mine/**")
	got := policy.Decide(res.Policy, policy.Request{
		Subject:  policy.Subject{Principals: []string{"user:alice"}},
		Action:   policy.ActionDownload,
		Resource: policy.Resource{Repo: "myrepo", Path: "mine/x"},
	})
	if got.MatchedRule != want {
		t.Errorf("MatchedRule = %q, want %q", got.MatchedRule, want)
	}
}

func walkGolden(t *testing.T, dir string, fn func(t *testing.T, path string)) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			fn(t, path)
		})
	}
}
