package memstore

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// UserRecord is a single user's credential and group membership, supplied at
// startup. Groups are full principal IDs (e.g. "group:engineers") and are
// stored verbatim. See docs/auth-design.md §4.1.
type UserRecord struct {
	Password string
	Groups   []string
}

// InMemoryAuthenticator matches HTTP Basic credentials against a static user
// map. It is the only Authenticator in v1; an OAuth or mTLS adapter would
// replace it without touching the PEP. See docs/auth-design.md §4.1 and §7.
type InMemoryAuthenticator struct {
	users map[string]UserRecord
}

// ErrInvalidCredentials is returned when Basic credentials are present but do
// not match a known user. The PEP maps any non-nil error to HTTP 401.
var ErrInvalidCredentials = errors.New("memstore: invalid credentials")

// NewInMemoryAuthenticator copies users so later caller mutation cannot
// change the live credential set.
func NewInMemoryAuthenticator(users map[string]UserRecord) *InMemoryAuthenticator {
	cp := make(map[string]UserRecord, len(users))
	for k, v := range users {
		cp[k] = v
	}
	return &InMemoryAuthenticator{users: cp}
}

// Authenticate resolves an HTTP request to a Subject.
//   - No Basic credentials: the anonymous subject, nil error.
//   - Valid credentials: Subject with "user:<name>" first, then the user's
//     groups verbatim.
//   - Present but invalid credentials: ErrInvalidCredentials (caller -> 401).
func (a *InMemoryAuthenticator) Authenticate(r *http.Request) (policy.Subject, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return policy.Subject{Principals: []string{policy.PrincipalAnonymous}}, nil
	}
	rec, found := a.users[username]
	// Always run the password comparison, even for an unknown user, and use a
	// constant-time compare so a wrong password is not distinguishable
	// byte-by-byte by timing. (Username-enumeration hardening is out of scope
	// for the in-memory v1.)
	expected := ""
	if found {
		expected = rec.Password
	}
	match := subtle.ConstantTimeCompare([]byte(password), []byte(expected)) == 1
	if !found || !match {
		return policy.Subject{}, ErrInvalidCredentials
	}
	principals := make([]string, 0, 1+len(rec.Groups))
	principals = append(principals, policy.PrincipalPrefixUser+username)
	principals = append(principals, rec.Groups...) // expands to a for loop over rec.Groups and appends each group to the principals slice
	// also, rec.Groups is a slice of strings, containing group:<group_name> strings
	return policy.Subject{Principals: principals}, nil
}

var _ ports.Authenticator = (*InMemoryAuthenticator)(nil)
