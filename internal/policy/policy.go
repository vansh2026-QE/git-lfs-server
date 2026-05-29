package policy

// Policy is the validated, in-memory authorization model produced by the
// Loader and consumed by the PDP. It is immutable while serving; hot
// reloads atomically swap the entire Policy via a pointer the PEP holds.
// See docs/auth-design.md §2 and §10.
type Policy struct {
	Repos map[string]*RepoPolicy
}

// RepoPolicy holds the per-principal grant tables for a single repository.
// Repos absent from Policy.Repos are ungoverned: the PEP short-circuits
// before consulting the PDP, and Decide called with an unknown repo fails
// closed.
type RepoPolicy struct {
	Principals map[string]*PrincipalGrants
}

// PrincipalGrants holds one segment trie per Action for a single principal.
// Actions with no grants are simply absent from Tries; absence means no
// permission, not silent allow.
type PrincipalGrants struct {
	Tries map[Action]*Trie
}

// Trie is the per-(principal, repo, action) segment trie of grant points.
// Operations and segment-matching logic live in trie.go.
// See docs/auth-design.md §6.
type Trie struct{}
