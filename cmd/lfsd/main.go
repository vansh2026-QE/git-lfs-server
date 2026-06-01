// Command lfsd is the custom Git LFS server binary. It wires the in-memory
// adapters from internal/memstore (and, later, production adapters from
// adapters/*) into the HTTP handlers exposed by internal/pep.
//
// See docs/server-scaffold.md and docs/auth-design.md for the design.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/vansh2026/git-lfs-server/internal/loader"
	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/pep"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("lfsd: config: %v", err)
	}
	if err := run(cfg); err != nil {
		log.Fatalf("lfsd: %v", err)
	}
}

// run loads the policy once, wires the adapters, and serves until the process
// is killed. One LocalFSObjectStore acts as both the ObjectStore (minting
// same-origin transfer URLs) and the BlobStore (storing the bytes); production
// would split these and drop the LocalServer wrapper.
func run(cfg Config) error {
	res, err := loader.New(memstore.NewFilePolicyStore(cfg.PolicyPath)).Load(context.Background())
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		log.Printf("lfsd: policy warning: %s", w.Message)
	}

	// Demo credentials. A real deployment swaps this Authenticator for an
	// OAuth/mTLS adapter without touching the PEP.
	auth := memstore.NewInMemoryAuthenticator(map[string]memstore.UserRecord{
		"alice": {Password: "alicepw"},
		"bob":   {Password: "bobpw"},
	})
	store := memstore.NewLocalFSObjectStore(cfg.StorageRoot, cfg.BaseURL)
	srv := pep.NewServer(auth, memstore.NewInMemoryPathIndex(), store, memstore.NewStderrAuditSink(), res.Policy)
	handler := pep.NewLocalServer(srv, store)

	log.Printf("lfsd: listening on %s (base-url=%s storage=%s policy=%s)",
		cfg.Addr, cfg.BaseURL, cfg.StorageRoot, cfg.PolicyPath)
	return http.ListenAndServe(cfg.Addr, handler)
}
