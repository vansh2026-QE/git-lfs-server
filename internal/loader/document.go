package loader

// policyDocument is the on-disk JSON shape. See docs/auth-design.md §10.2.
type policyDocument struct {
	Version     int                     `json:"version"`
	Memberships map[string][]string     `json:"memberships"`
	Repos       map[string]repoDocument `json:"repos"`
}

type repoDocument struct {
	Paths map[string]pathACLDocument `json:"paths"`
}

// pathACLDocument maps a principal ID -> SVN-style access code ("r", "w",
// "rw"). The loader transposes this path-centric shape into the internal
// per-principal tries. See docs/auth-design.md §10.2.
type pathACLDocument map[string]string
