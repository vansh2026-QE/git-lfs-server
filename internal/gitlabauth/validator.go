// Package gitlabauth provides an Authenticator adapter that validates Git LFS
// requests against a GitLab instance. It accepts both CLI Personal Access
// Tokens (delivered as the HTTP Basic password) and reviewer OAuth2 access
// tokens (delivered as a Bearer header); both are validated identically by
// calling GitLab's GET /api/v4/user. Resolved identities are cached for a
// short TTL. See docs/auth-design.md §4.1.
package gitlabauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrInvalidToken is returned when GitLab rejects the token (401/403). The
// authenticator maps this to ErrInvalidCredentials, which the PEP turns into
// HTTP 401. Other (infrastructure) failures are returned wrapped and distinct.
var ErrInvalidToken = errors.New("gitlabauth: token rejected by gitlab")

// Validator resolves a raw GitLab token to a username by calling the GitLab
// REST API. It holds no per-token state; caching is layered on top.
type Validator struct {
	baseURL string
	client  *http.Client
}

// NewValidator builds a Validator for the given GitLab base URL (e.g.
// "https://gitlab.example.com"). A trailing slash is trimmed. If client is
// nil, a client with a sane timeout is used.
func NewValidator(baseURL string, client *http.Client) *Validator {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Validator{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

// userResponse is the subset of GET /api/v4/user lfsd consumes.
type userResponse struct {
	Username string `json:"username"`
}

// Validate calls GET {base}/api/v4/user with the token as a Bearer credential
// (GitLab accepts both PATs and OAuth2 access tokens this way) and returns the
// resolved username.
//
//   - 200 with a non-empty username -> (username, nil)
//   - 401/403                       -> ("", ErrInvalidToken)
//   - any other status / transport  -> ("", wrapped infrastructure error)
func (v *Validator) Validate(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/v4/user", nil)
	if err != nil {
		return "", fmt.Errorf("gitlabauth: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlabauth: call gitlab: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		var u userResponse
		if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
			return "", fmt.Errorf("gitlabauth: decode user response: %w", err)
		}
		if u.Username == "" {
			return "", fmt.Errorf("gitlabauth: gitlab returned an empty username")
		}
		return u.Username, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", ErrInvalidToken
	default:
		return "", fmt.Errorf("gitlabauth: unexpected gitlab status %d", resp.StatusCode)
	}
}
