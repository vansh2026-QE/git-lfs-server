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

	"github.com/vansh2026/git-lfs-server/internal/gitlabauth"
	"github.com/vansh2026/git-lfs-server/internal/loader"
	"github.com/vansh2026/git-lfs-server/internal/memstore"
	"github.com/vansh2026/git-lfs-server/internal/pep"
	"github.com/vansh2026/git-lfs-server/internal/ports"
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

	auth, requireAuth, err := buildAuthenticator(cfg)
	if err != nil {
		return err
	}
	index, err := memstore.NewFilePathIndex(cfg.IndexPath)
	if err != nil {
		return fmt.Errorf("lfsd: load path index %q: %w", cfg.IndexPath, err)
	}
	audit := memstore.NewStderrAuditSink()

	var handler http.Handler
	switch cfg.StorageBackend {
	case "local":
		store := memstore.NewLocalFSObjectStore(cfg.StorageRoot, cfg.BaseURL)
		srv := pep.NewServer(auth, index, store, audit, res.Policy)
		srv.SetRequireAuth(requireAuth)
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
		srv := pep.NewServer(auth, index, store, audit, res.Policy)
		srv.SetRequireAuth(requireAuth)
		handler = srv
	default:
		return fmt.Errorf("lfsd: unknown storage backend %q (want local or s3)", cfg.StorageBackend)
	}

	if len(cfg.CORSOrigins) > 0 {
		handler = pep.CORS(cfg.CORSOrigins, handler)
	}

	log.Printf("lfsd: listening on %s (backend=%s auth=%s base-url=%s policy=%s index=%s cors=%v)",
		cfg.Addr, cfg.StorageBackend, cfg.AuthBackend, cfg.BaseURL, cfg.PolicyPath, cfg.IndexPath, cfg.CORSOrigins)
	return http.ListenAndServe(cfg.Addr, handler)
}

// buildAuthenticator selects the Authenticator from config. The "memory"
// backend keeps the open demo credentials (anonymous allowed, policy decides);
// the "gitlab" backend validates PAT/OAuth2 tokens and makes auth mandatory so
// git-lfs is prompted for credentials. See docs/auth-design.md §4.1 and §7.
func buildAuthenticator(cfg Config) (auth ports.Authenticator, requireAuth bool, err error) {
	switch cfg.AuthBackend {
	case "memory":
		return memstore.NewInMemoryAuthenticator(map[string]memstore.UserRecord{
			"alice": {Password: "alicepw"},
			"bob":   {Password: "bobpw"},
		}), false, nil
	case "gitlab":
		if cfg.GitLabBaseURL == "" {
			return nil, false, fmt.Errorf("lfsd: auth-backend gitlab requires -gitlab-base-url")
		}
		return gitlabauth.New(cfg.GitLabBaseURL, cfg.AuthCacheTTL, cfg.AuthCacheNegativeTTL), true, nil
	default:
		return nil, false, fmt.Errorf("lfsd: unknown auth backend %q (want memory or gitlab)", cfg.AuthBackend)
	}
}
