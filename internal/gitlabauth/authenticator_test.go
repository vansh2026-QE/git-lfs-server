package gitlabauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/policy"
)

// fakeGitLab returns an httptest server standing in for GET /api/v4/user. It
// accepts goodToken (resolving to wantUser) and rejects everything else, and
// counts how many times it was hit so cache behavior is observable.
func fakeGitLab(t *testing.T, goodToken, wantUser string, calls *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(calls, 1)
		if r.URL.Path != "/api/v4/user" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+goodToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"username": wantUser})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestAuth(baseURL string, posTTL, negTTL time.Duration) *Authenticator {
	return &Authenticator{
		validator: NewValidator(baseURL, nil),
		cache:     newTokenCache(posTTL, negTTL),
	}
}

func TestAuthenticate_Transports(t *testing.T) {
	const goodToken, wantUser = "good-token", "alice"
	var calls int32
	srv := fakeGitLab(t, goodToken, wantUser, &calls)

	cases := []struct {
		name      string
		setup     func(*http.Request)
		wantErr   bool
		wantPrins []string
	}{
		{
			name:      "bearer token (reviewer oauth2)",
			setup:     func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+goodToken) },
			wantPrins: []string{"user:alice"},
		},
		{
			name:      "basic password (cli pat)",
			setup:     func(r *http.Request) { r.SetBasicAuth("oauth2", goodToken) },
			wantPrins: []string{"user:alice"},
		},
		{
			name:      "no credentials is anonymous",
			setup:     func(r *http.Request) {},
			wantPrins: []string{policy.PrincipalAnonymous},
		},
		{
			name:    "invalid bearer token errors",
			setup:   func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") },
			wantErr: true,
		},
		{
			name:    "invalid basic password errors",
			setup:   func(r *http.Request) { r.SetBasicAuth("oauth2", "nope") },
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth := newTestAuth(srv.URL, time.Minute, time.Minute)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setup(req)
			sub, err := auth.Authenticate(req)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(sub.Principals, tc.wantPrins) {
				t.Errorf("principals: got %v, want %v", sub.Principals, tc.wantPrins)
			}
		})
	}
}

func TestAuthenticate_PositiveCacheAvoidsSecondCall(t *testing.T) {
	const goodToken = "good-token"
	var calls int32
	srv := fakeGitLab(t, goodToken, "alice", &calls)
	auth := newTestAuth(srv.URL, time.Minute, time.Minute)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+goodToken)
		if _, err := auth.Authenticate(req); err != nil {
			t.Fatalf("call %d errored: %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("gitlab hit %d times, want 1 (cache should serve the rest)", got)
	}
}

func TestAuthenticate_NegativeCacheAvoidsSecondCall(t *testing.T) {
	const goodToken = "good-token"
	var calls int32
	srv := fakeGitLab(t, goodToken, "alice", &calls)
	auth := newTestAuth(srv.URL, time.Minute, time.Minute)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		if _, err := auth.Authenticate(req); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("call %d: got %v, want ErrInvalidCredentials", i, err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("gitlab hit %d times, want 1 (negative cache should serve the rest)", got)
	}
}

func TestValidator_Statuses(t *testing.T) {
	const goodToken, wantUser = "good-token", "carol"
	var calls int32
	srv := fakeGitLab(t, goodToken, wantUser, &calls)
	v := NewValidator(srv.URL, nil)

	t.Run("valid token resolves username", func(t *testing.T) {
		got, err := v.Validate(context.Background(), goodToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != wantUser {
			t.Errorf("username: got %q, want %q", got, wantUser)
		}
	})

	t.Run("rejected token is ErrInvalidToken", func(t *testing.T) {
		if _, err := v.Validate(context.Background(), "nope"); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("got %v, want ErrInvalidToken", err)
		}
	})
}
