package policy

import "testing"

// buildPolicy constructs a one-repo Policy from a nested description:
// principal -> action -> {path: grantID}. Grants are inserted into per-action
// tries via the real Insert path.
func buildPolicy(t *testing.T, repo string, grants map[string]map[Action]map[string]string) *Policy {
	t.Helper()
	rp := &RepoPolicy{Principals: map[string]*PrincipalGrants{}}
	for principal, actions := range grants {
		pg := &PrincipalGrants{Tries: map[Action]*Trie{}}
		for action, paths := range actions {
			tr := NewTrie()
			for path, id := range paths {
				if _, _, err := tr.Insert(path, GrantID(id)); err != nil {
					t.Fatalf("Insert(%q): %v", path, err)
				}
			}
			pg.Tries[action] = tr
		}
		rp.Principals[principal] = pg
	}
	return &Policy{Repos: map[string]*RepoPolicy{repo: rp}}
}

func req(principals []string, action Action, repo, path string) Request {
	return Request{
		Subject:  Subject{Principals: principals},
		Action:   action,
		Resource: Resource{Repo: repo, Path: path},
	}
}

func TestDecide(t *testing.T) {
	pol := buildPolicy(t, "myrepo", map[string]map[Action]map[string]string{
		"user:alice": {
			ActionDownload: {"**": "alice-dl"},
			ActionUpload:   {"mine/**": "alice-up"},
		},
		"group:engineers": {
			ActionUpload: {"src/**": "eng-up"},
		},
	})

	cases := []struct {
		name    string
		req     Request
		wantEff Effect
		wantSrc string
		wantID  GrantID
	}{
		{
			"user grant matches",
			req([]string{"user:alice", "group:engineers"}, ActionDownload, "myrepo", "anything/x"),
			Permit, "user:alice", "alice-dl",
		},
		{
			"falls through to group grant",
			req([]string{"user:alice", "group:engineers"}, ActionUpload, "myrepo", "src/main.go"),
			Permit, "group:engineers", "eng-up",
		},
		{
			"user grant preferred over group when both match",
			req([]string{"user:alice", "group:engineers"}, ActionUpload, "myrepo", "mine/x"),
			Permit, "user:alice", "alice-up",
		},
		{
			"no principal grants the path",
			req([]string{"user:alice", "group:engineers"}, ActionUpload, "myrepo", "secret/db"),
			Deny, "", "",
		},
		{
			"unknown repo fails closed",
			req([]string{"user:alice"}, ActionDownload, "otherrepo", "x"),
			Deny, "", "",
		},
		{
			"principal not in policy denies",
			req([]string{"user:carol"}, ActionDownload, "myrepo", "x"),
			Deny, "", "",
		},
		{
			"action with no trie denies",
			req([]string{"group:engineers"}, ActionDownload, "myrepo", "src/x"),
			Deny, "", "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Decide(pol, tc.req)
			if got.Effect != tc.wantEff || got.Source != tc.wantSrc || got.MatchedRule != tc.wantID {
				t.Errorf("Decide() = {eff:%v src:%q rule:%q}, want {eff:%v src:%q rule:%q}",
					got.Effect, got.Source, got.MatchedRule, tc.wantEff, tc.wantSrc, tc.wantID)
			}
		})
	}
}
