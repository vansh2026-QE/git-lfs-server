package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the lfsd runtime settings, sourced from flags with environment
// variable fallbacks. The defaults run a self-contained demo server on
// localhost with on-disk storage under ./lfs-data.
type Config struct {
	Addr        string // TCP listen address, e.g. ":8080"
	BaseURL     string // public origin used to mint transfer hrefs
	StorageRoot string // directory the BlobStore writes objects under (local backend)
	PolicyPath  string // path to the JSON policy document
	IndexPath   string // JSON file persisting the (repo, oid) -> paths index
	MessagePath string // JSON file persisting redacted commit messages + bound paths

	// StorageBackend selects the object backend: "local" (on-disk BlobStore
	// plus the open transfer endpoints) or "s3" (presigned direct-to-bucket
	// transfers; no BlobStore, no transfer endpoints).
	StorageBackend string

	// AuthBackend selects the Authenticator: "memory" (static demo creds, the
	// default) or "gitlab" (validate PAT/OAuth2 tokens against a GitLab
	// instance). See docs/auth-design.md §4.1.
	AuthBackend string

	// GitLabBaseURL is the GitLab instance lfsd validates tokens against, e.g.
	// "https://gitlab.example.com". Used only when AuthBackend == "gitlab".
	GitLabBaseURL string

	// AuthCacheTTL bounds how long a successfully resolved token -> Subject
	// stays cached before lfsd re-validates against GitLab.
	AuthCacheTTL time.Duration

	// AuthCacheNegativeTTL bounds how long a known-bad token is remembered as
	// invalid, throttling repeated lookups for the same rejected token.
	AuthCacheNegativeTTL time.Duration

	// CORSOrigins is the allowlist of browser origins permitted to call lfsd
	// cross-origin (e.g. the reviewer extension's "chrome-extension://<id>").
	// Empty disables CORS. See docs/auth-design.md §4.1.1.
	CORSOrigins []string

	// S3 settings are used only when StorageBackend == "s3".
	S3Bucket          string
	S3Region          string
	S3Endpoint        string // custom endpoint for MinIO/R2; empty means AWS
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool // path-style addressing; required for MinIO
	S3Prefix          string

	// URLExpiry bounds how long a minted transfer URL stays valid.
	URLExpiry time.Duration
}

// parseConfig builds a Config from args. Each flag falls back to an
// environment variable, then to a demo-friendly default.
func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("lfsd", flag.ContinueOnError)
	var c Config
	fs.StringVar(&c.Addr, "addr", envOr("LFSD_ADDR", ":8080"), "TCP listen address")
	fs.StringVar(&c.BaseURL, "base-url", envOr("LFSD_BASE_URL", "http://localhost:8080"), "public origin for minted transfer URLs")
	fs.StringVar(&c.StorageRoot, "storage", envOr("LFSD_STORAGE", "./lfs-data"), "directory for stored objects (local backend)")
	fs.StringVar(&c.PolicyPath, "policy", envOr("LFSD_POLICY", "examples/policy.json"), "path to the JSON policy document")
	fs.StringVar(&c.IndexPath, "pathindex", envOr("LFSD_PATHINDEX", "./lfs-data/pathindex.json"), "JSON file persisting the (repo, oid) -> paths index")
	fs.StringVar(&c.MessagePath, "messages", envOr("LFSD_MESSAGES", "./lfs-data/messages.json"), "JSON file persisting redacted commit messages and their bound paths")
	fs.StringVar(&c.StorageBackend, "storage-backend", envOr("LFSD_STORAGE_BACKEND", "local"), "object backend: local or s3")
	fs.StringVar(&c.AuthBackend, "auth-backend", envOr("LFSD_AUTH_BACKEND", "memory"), "authenticator: memory or gitlab")
	fs.StringVar(&c.GitLabBaseURL, "gitlab-base-url", envOr("LFSD_GITLAB_BASE_URL", ""), "GitLab base URL for token validation (gitlab backend)")
	fs.DurationVar(&c.AuthCacheTTL, "auth-cache-ttl", envDur("LFSD_AUTH_CACHE_TTL", 5*time.Minute), "how long a resolved token stays cached (gitlab backend)")
	fs.DurationVar(&c.AuthCacheNegativeTTL, "auth-cache-negative-ttl", envDur("LFSD_AUTH_CACHE_NEGATIVE_TTL", 30*time.Second), "how long a rejected token is cached as invalid (gitlab backend)")
	fs.StringVar(&c.S3Bucket, "s3-bucket", envOr("LFSD_S3_BUCKET", "lfs-objects"), "S3 bucket name")
	fs.StringVar(&c.S3Region, "s3-region", envOr("LFSD_S3_REGION", "us-east-1"), "S3 region (any non-empty value for MinIO)")
	fs.StringVar(&c.S3Endpoint, "s3-endpoint", envOr("LFSD_S3_ENDPOINT", ""), "custom S3 endpoint for MinIO/R2; empty for AWS")
	fs.StringVar(&c.S3AccessKeyID, "s3-access-key-id", envOr("LFSD_S3_ACCESS_KEY_ID", ""), "S3 access key id")
	fs.StringVar(&c.S3SecretAccessKey, "s3-secret-access-key", envOr("LFSD_S3_SECRET_ACCESS_KEY", ""), "S3 secret access key")
	fs.BoolVar(&c.S3UsePathStyle, "s3-use-path-style", envBool("LFSD_S3_USE_PATH_STYLE", false), "use path-style addressing (required for MinIO)")
	fs.StringVar(&c.S3Prefix, "s3-prefix", envOr("LFSD_S3_PREFIX", ""), "optional key prefix for stored objects")
	fs.DurationVar(&c.URLExpiry, "url-expiry", envDur("LFSD_URL_EXPIRY", 10*time.Minute), "how long minted transfer URLs remain valid")
	var corsRaw string
	fs.StringVar(&corsRaw, "cors-origins", envOr("LFSD_CORS_ORIGINS", ""), "comma-separated browser origins allowed to call lfsd cross-origin (empty disables CORS)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	c.CORSOrigins = splitCSV(corsRaw)
	return c, nil
}

// splitCSV splits a comma-separated list into trimmed, non-empty entries.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
