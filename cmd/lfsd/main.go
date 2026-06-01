// Command lfsd is the custom Git LFS server binary. It wires the in-memory
// adapters from internal/memstore (and, later, production adapters from
// adapters/*) into the HTTP handlers exposed by internal/pep.
//
// See docs/server-scaffold.md and docs/auth-design.md for the design.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/vansh2026/git-lfs-server/internal/loader"
	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/pep"
	"github.com/vansh2026/git-lfs-server/internal/s3store"
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

// run loads the policy once, wires the adapters for the configured storage
// backend, and serves until the process is killed. The "local" backend uses a
// LocalFSObjectStore as both ObjectStore (minting same-origin URLs) and
// BlobStore (storing bytes), wrapped by a LocalServer that exposes the open
// transfer endpoints. The "s3" backend mints pre-signed URLs only: the client
// transfers bytes directly to the bucket, so there is no BlobStore and the
// plain batch Server is served. See docs/auth-design.md §4.5.
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
	index := memstore.NewInMemoryPathIndex()
	audit := memstore.NewStderrAuditSink()

	var handler http.Handler
	switch cfg.StorageBackend {
	case "local":
		store := memstore.NewLocalFSObjectStore(cfg.StorageRoot, cfg.BaseURL)
		srv := pep.NewServer(auth, index, store, audit, res.Policy)
		handler = pep.NewLocalServer(srv, store)
	case "s3":
		store, serr := s3store.New(context.Background(), s3store.Config{
			Bucket:          cfg.S3Bucket,
			Region:          cfg.S3Region,
			Endpoint:        cfg.S3Endpoint,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
			UsePathStyle:    cfg.S3UsePathStyle,
			Prefix:          cfg.S3Prefix,
			URLExpiry:       cfg.URLExpiry,
		})
		if serr != nil {
			return serr
		}
		handler = pep.NewServer(auth, index, store, audit, res.Policy)
	default:
		return fmt.Errorf("lfsd: unknown storage backend %q (want local or s3)", cfg.StorageBackend)
	}

	log.Printf("lfsd: listening on %s (backend=%s base-url=%s policy=%s)",
		cfg.Addr, cfg.StorageBackend, cfg.BaseURL, cfg.PolicyPath)
	return http.ListenAndServe(cfg.Addr, handler)
}
