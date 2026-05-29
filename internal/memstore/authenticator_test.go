package memstore_test

import (
	"testing"

	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/ports"
	"github.com/vansh2026/git-lfs-server/internal/ports/portstest"
)

func TestInMemoryAuthenticator_Contract(t *testing.T) {
	portstest.RunAuthenticatorContract(t, func(users []portstest.AuthUser) ports.Authenticator {
		recs := make(map[string]memstore.UserRecord, len(users))
		for _, u := range users {
			recs[u.Username] = memstore.UserRecord{Password: u.Password, Groups: u.Groups}
		}
		return memstore.NewInMemoryAuthenticator(recs)
	})
}
