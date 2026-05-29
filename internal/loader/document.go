package loader

// policyDocument is the on-disk JSON shape. See docs/auth-design.md §10.2.
type policyDocument struct {
	Version     int                     `json:"version"`
	Memberships map[string][]string     `json:"memberships"`
	Repos       map[string]repoDocument `json:"repos"`
}

type repoDocument struct {
	Principals map[string]principalGrantDocument `json:"principals"`
}

// principalGrantDocument maps action name -> grant path patterns.
type principalGrantDocument map[string][]string
