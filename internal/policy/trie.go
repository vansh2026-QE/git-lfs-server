package policy

import (
	"fmt"
	"strings"
)

// Trie is the per-(principal, repo, action) segment trie of grant points.
// See docs/auth-design.md §6.
type Trie struct {
	root *trieNode
}

type trieNode struct {
	grant    GrantID // "" means not a grant point
	subtree  bool    // true: prefix/** or ** (matches this node and all below); false: exact path (matches only when the path ends here)
	children map[string]*trieNode
}

// NewTrie returns an empty trie with no grants.
func NewTrie() *Trie {
	return &Trie{root: &trieNode{}}
}

// Insert marks path as a granted subtree and returns the assigned grant ID.
// If an existing ancestor grant already covers path, the insert is a no-op:
// effectiveID is the subsuming grant's ID and warning is non-empty (§6.3,
// ancestor-only). A path with '..' or a misplaced '**' is a load-time error.
func (t *Trie) Insert(path string, id GrantID) (effectiveID GrantID, warning string, err error) {
	segs, subtree, err := grantSegments(path)
	if err != nil {
		return "", "", err
	}
	node := t.root
	for _, seg := range segs {
		// Only a subtree grant on an ancestor covers what we are inserting;
		// an exact grant matches solely its own node.
		if node.grant != "" && node.subtree {
			return node.grant, subsumed(id, node.grant), nil
		}
		if node.children == nil {
			node.children = make(map[string]*trieNode)
		}
		child, ok := node.children[seg]
		if !ok {
			child = &trieNode{}
			node.children[seg] = child
		}
		node = child
	}
	if node.grant != "" {
		return node.grant, subsumed(id, node.grant), nil
	}
	node.grant = id
	node.subtree = subtree
	return id, "", nil
}

// Decide walks path from the root and returns the first grant ID encountered.
// A malformed path (containing '..') fails closed: ("", false).
func (t *Trie) Decide(path string) (GrantID, bool) {
	segs, err := splitPath(path)
	if err != nil {
		return "", false
	}
	node := t.root
	if node.grant != "" && node.subtree {
		return node.grant, true
	}
	for i, seg := range segs {
		child, ok := node.children[seg]
		if !ok {
			return "", false
		}
		node = child
		if node.grant != "" && (node.subtree || i == len(segs)-1) {
			return node.grant, true
		}
	}
	return "", false
}

func subsumed(newID, byID GrantID) string {
	return fmt.Sprintf("grant %q subsumed by existing grant %q", newID, byID)
}

// grantSegments normalizes a grant path and reports whether it is a subtree
// grant. "**" is the root subtree (zero segments); a trailing "/**" is a
// subtree grant on the prefix node; otherwise the path is an exact grant.
// '**' anywhere but the final segment, and any '..', are rejected.
func grantSegments(path string) (segs []string, subtree bool, err error) {
	switch {
	case path == "**":
		return []string{}, true, nil
	case strings.HasSuffix(path, "/**"):
		path = strings.TrimSuffix(path, "/**")
		subtree = true
	}
	segs, err = splitPath(path)
	if err != nil {
		return nil, false, err
	}
	for _, s := range segs {
		if s == "**" {
			return nil, false, fmt.Errorf("policy: '**' may only be the final segment: %q", path)
		}
	}
	return segs, subtree, nil
}

// splitPath splits on '/', drops empty segments, and rejects '..'.
func splitPath(path string) ([]string, error) {
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		switch p {
		case "":
			continue
		case "..":
			return nil, fmt.Errorf("policy: '..' not allowed in path %q", path)
		}
		out = append(out, p)
	}
	return out, nil
}
