package portstest

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// AuthUser is a fixture credential the contract asks the factory to install
// into the Authenticator under test. Groups are full principal IDs.
type AuthUser struct {
	Username string
	Password string
	Groups   []string
}

// RunAuthenticatorContract exercises the ports.Authenticator contract. The
// factory must return an Authenticator that recognises exactly the supplied
// users. See docs/auth-design.md §4.1 and §7.
func RunAuthenticatorContract(t *testing.T, factory func(users []AuthUser) ports.Authenticator) {
	t.Helper()

	users := []AuthUser{
		{Username: "alice", Password: "secret", Groups: []string{"group:engineers"}},
	}

	t.Run("NoCredentialsReturnAnonymous", func(t *testing.T) {
		auth := factory(users)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		sub, err := auth.Authenticate(req)
		if err != nil {
			t.Fatalf("anonymous must not error: %v", err)
		}
		if !slices.Equal(sub.Principals, []string{policy.PrincipalAnonymous}) {
			t.Errorf("got %v, want [%s]", sub.Principals, policy.PrincipalAnonymous)
		}
	})

	t.Run("ValidCredentialsReturnUserAndGroups", func(t *testing.T) {
		auth := factory(users)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("alice", "secret")
		sub, err := auth.Authenticate(req)
		if err != nil {
			t.Fatalf("valid creds errored: %v", err)
		}
		want := []string{"user:alice", "group:engineers"}
		if !slices.Equal(sub.Principals, want) {
			t.Errorf("got %v, want %v", sub.Principals, want)
		}
	})

	t.Run("UserPrincipalComesFirst", func(t *testing.T) {
		auth := factory(users)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("alice", "secret")
		sub, err := auth.Authenticate(req)
		if err != nil {
			t.Fatal(err)
		}
		if len(sub.Principals) == 0 || sub.Principals[0] != "user:alice" {
			t.Errorf("user principal must be first, got %v", sub.Principals)
		}
	})

	t.Run("WrongPasswordIsError", func(t *testing.T) {
		auth := factory(users)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("alice", "wrong")
		if _, err := auth.Authenticate(req); err == nil {
			t.Error("expected error for wrong password")
		}
	})

	t.Run("UnknownUserIsError", func(t *testing.T) {
		auth := factory(users)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("bob", "secret")
		if _, err := auth.Authenticate(req); err == nil {
			t.Error("expected error for unknown user")
		}
	})
}
