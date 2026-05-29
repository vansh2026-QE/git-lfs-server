package loader

import "github.com/vansh2026/git-lfs-server/internal/policy"

// LoadResult is the output of a successful policy load. Memberships are a
// sidecar for the PEP (identity), not part of the PDP model. See
// docs/auth-design.md §10.4.
type LoadResult struct {
	Policy      *policy.Policy
	Memberships map[string][]string
	Warnings    []Warning
}

// Warning records a non-fatal issue encountered during load (subsumed grant,
// undefined group reference, etc.). The server may start with warnings present.
type Warning struct {
	Message string
}
