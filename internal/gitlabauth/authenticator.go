package gitlabauth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// ErrInvalidCredentials is returned when a token is present but GitLab rejects
// it. The PEP maps any non-nil Authenticate error to HTTP 401.
var ErrInvalidCredentials = errors.New("gitlabauth: invalid credentials")

// tokenValidator resolves a raw token to a GitLab username. *Validator
// satisfies it; tests substitute a fake.
type tokenValidator interface {
	Validate(ctx context.Context, token string) (string, error)
}

// Authenticator validates GitLab tokens and maps them to a user-only Subject.
// It accepts a token either as an OAuth2 Bearer credential (reviewer extension)
// or as the HTTP Basic password (CLI Personal Access Token via git's credential
// helper), then resolves it to "user:<gitlab-username>". Successful and
// rejected validations are cached for their respective TTLs. See
// docs/auth-design.md §4.1.
type Authenticator struct {
	validator tokenValidator
	cache     *tokenCache
}

// New builds an Authenticator that validates tokens against the GitLab instance
// at baseURL, caching resolved identities for cacheTTL and rejected tokens for
// negativeTTL.
func New(baseURL string, cacheTTL, negativeTTL time.Duration) *Authenticator {
	return &Authenticator{
		validator: NewValidator(baseURL, nil),
		cache:     newTokenCache(cacheTTL, negativeTTL),
	}
}

// Authenticate resolves an HTTP request to a Subject.
//   - No token: the anonymous subject, nil error.
//   - Valid token: Subject{Principals: ["user:<username>"]}.
//   - Present but rejected token: ErrInvalidCredentials (caller -> 401).
//   - Infrastructure failure reaching GitLab: a wrapped error (caller -> 401).
func (a *Authenticator) Authenticate(r *http.Request) (policy.Subject, error) {
	token, ok := extractToken(r)
	if !ok {
		return policy.Subject{Principals: []string{policy.PrincipalAnonymous}}, nil
	}

	if e, hit := a.cache.get(token); hit {
		if e.valid {
			return subjectFor(e.username), nil
		}
		return policy.Subject{}, ErrInvalidCredentials
	}

	username, err := a.validator.Validate(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			a.cache.putInvalid(token)
			return policy.Subject{}, ErrInvalidCredentials
		}
		// Infrastructure failure (timeout, 5xx): do not cache; fail closed.
		return policy.Subject{}, err
	}

	a.cache.putValid(token, username)
	return subjectFor(username), nil
}

// extractToken pulls the raw token from a request. A Bearer Authorization
// header wins (OAuth2 reviewer); otherwise the HTTP Basic password is treated
// as the token (CLI PAT). The Basic username is ignored by convention.
func extractToken(r *http.Request) (string, bool) {
	if h := r.Header.Get("Authorization"); h != "" {
		if len(h) >= 7 && strings.EqualFold(h[:7], "Bearer ") {
			if tok := strings.TrimSpace(h[7:]); tok != "" {
				return tok, true
			}
			return "", false
		}
	}
	if _, password, ok := r.BasicAuth(); ok && password != "" {
		return password, true
	}
	return "", false
}

func subjectFor(username string) policy.Subject {
	return policy.Subject{Principals: []string{policy.PrincipalPrefixUser + username}}
}

var _ ports.Authenticator = (*Authenticator)(nil)
