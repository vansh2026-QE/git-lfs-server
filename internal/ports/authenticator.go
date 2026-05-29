package ports

import (
	"net/http"

	"github.com/vansh2026/git-lfs-server/internal/policy"
)

// Authenticator resolves an HTTP request to the acting Subject.
//
// Contract:
//   - Returns a fully-resolved Subject including transitively-flattened
//     group memberships.
//   - Anonymous access returns Subject{Principals:
//     []string{policy.PrincipalAnonymous}} with a nil error — not an error.
//   - A non-nil error must be treated by the caller as HTTP 401. The
//     Authenticator must not encode policy/authorisation decisions in its
//     errors; that is the PDP's job.
//
// See docs/auth-design.md §4.1.
type Authenticator interface {
	Authenticate(r *http.Request) (policy.Subject, error)
}
