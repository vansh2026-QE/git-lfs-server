package policy

import "testing"

// insertOrFatal inserts a grant and fails the test on a normalization error.
func insertOrFatal(t *testing.T, tr *Trie, path, id string) {
	t.Helper()
	if _, _, err := tr.Insert(path, GrantID(id)); err != nil {
		t.Fatalf("Insert(%q, %q): unexpected error %v", path, id, err)
	}
}

func TestTrieDecide(t *testing.T) {
	cases := []struct {
		name   string
		grants [][2]string // {path, id}
		query  string
		wantID string
		wantOK bool
	}{
		{"root grant matches anything", [][2]string{{"**", "g-root"}}, "any/deep/path", "g-root", true},
		{"subtree grant matches child", [][2]string{{"public/**", "g1"}}, "public/drafts/foo.bin", "g1", true},
		{"subtree grant is segment-wise", [][2]string{{"public/**", "g1"}}, "publicize-secret.txt", "", false},
		{"exact path matches itself", [][2]string{{"a/b/c.bin", "g1"}}, "a/b/c.bin", "g1", true},
		{"exact path does not match prefix", [][2]string{{"a/b/c.bin", "g1"}}, "a/b", "", false},
		{"exact path does not match deeper", [][2]string{{"a/b/c.bin", "g1"}}, "a/b/c.bin/d", "", false},
		{"no grant denies", [][2]string{{"src/**", "g1"}}, "docs/x", "", false},
		{"shallower grant wins on walk", [][2]string{{"a/**", "shallow"}, {"a/b/c", "deep"}}, "a/b/c", "shallow", true},
		{"normalized query matches", [][2]string{{"public/**", "g1"}}, "/public//drafts/", "g1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTrie()
			for _, g := range tc.grants {
				insertOrFatal(t, tr, g[0], g[1])
			}
			gotID, gotOK := tr.Decide(tc.query)
			if string(gotID) != tc.wantID || gotOK != tc.wantOK {
				t.Errorf("Decide(%q) = (%q, %v), want (%q, %v)", tc.query, gotID, gotOK, tc.wantID, tc.wantOK)
			}
		})
	}
}

func TestTrieInsertSubsumedByAncestor(t *testing.T) {
	tr := NewTrie()
	insertOrFatal(t, tr, "public/**", "g1")

	effectiveID, warning, err := tr.Insert("public/drafts/**", "g2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warning == "" {
		t.Error("expected a subsumption warning, got none")
	}
	if effectiveID != "g1" {
		t.Errorf("effectiveID = %q, want %q (the subsuming ancestor)", effectiveID, "g1")
	}
	// The subsumed grant must not have created a second matching node.
	if id, ok := tr.Decide("public/drafts/x"); !ok || id != "g1" {
		t.Errorf("Decide after subsumed insert = (%q, %v), want (g1, true)", id, ok)
	}
}

func TestTrieInsertRejectsTraversal(t *testing.T) {
	tr := NewTrie()
	if _, _, err := tr.Insert("a/../b/**", "g1"); err == nil {
		t.Error("expected error inserting a grant path containing '..'")
	}
}

func TestTrieDecideMalformedDenies(t *testing.T) {
	tr := NewTrie()
	insertOrFatal(t, tr, "**", "g-root")
	if id, ok := tr.Decide("a/../b"); ok || id != "" {
		t.Errorf("Decide of traversal path = (%q, %v), want (\"\", false)", id, ok)
	}
}
