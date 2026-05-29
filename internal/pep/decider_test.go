package pep_test

import (
	"context"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/loader"
	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/pep"
	"github.com/vansh2026/git-lfs-server/internal/policy"
)

func loadPolicy(t *testing.T, doc string) *policy.Policy {
	t.Helper()
	res, err := loader.New(memstore.NewStringPolicyStore(doc)).Load(context.Background())
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	return res.Policy
}

func TestDecideAll(t *testing.T) {
	p := loadPolicy(t, `{
		"version": 1,
		"repos": {"r": {"principals": {
			"user:alice": {"download": ["pub/**"], "upload": ["mine/**"]}
		}}}
	}`)
	alice := policy.Subject{Principals: []string{"user:alice"}}

	t.Run("download mixes permit and deny in order", func(t *testing.T) {
		objs := []pep.Verified{
			{OID: "o1", Path: "pub/a.bin"},
			{OID: "o2", Path: "secret/b.bin"},
		}
		got := pep.DecideAll(p, alice, "r", pep.OpDownload, objs)
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].Effect != policy.Permit {
			t.Errorf("obj0 = %v, want Permit", got[0].Effect)
		}
		if got[1].Effect != policy.Deny {
			t.Errorf("obj1 = %v, want Deny", got[1].Effect)
		}
	})

	t.Run("operation selects the action", func(t *testing.T) {
		objs := []pep.Verified{{OID: "o", Path: "mine/x.bin"}}
		if d := pep.DecideAll(p, alice, "r", pep.OpUpload, objs); d[0].Effect != policy.Permit {
			t.Errorf("upload mine/x = %v, want Permit", d[0].Effect)
		}
		if d := pep.DecideAll(p, alice, "r", pep.OpDownload, objs); d[0].Effect != policy.Deny {
			t.Errorf("download mine/x = %v, want Deny", d[0].Effect)
		}
	})

	t.Run("empty input yields empty decisions", func(t *testing.T) {
		if got := pep.DecideAll(p, alice, "r", pep.OpDownload, nil); len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})
}
