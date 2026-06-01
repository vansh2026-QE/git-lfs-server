package main

import (
	"flag"
	"os"
	"strconv"
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

	// StorageBackend selects the object backend: "local" (on-disk BlobStore
	// plus the open transfer endpoints) or "s3" (presigned direct-to-bucket
	// transfers; no BlobStore, no transfer endpoints).
	StorageBackend string

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
	fs.StringVar(&c.StorageBackend, "storage-backend", envOr("LFSD_STORAGE_BACKEND", "local"), "object backend: local or s3")
	fs.StringVar(&c.S3Bucket, "s3-bucket", envOr("LFSD_S3_BUCKET", "lfs-objects"), "S3 bucket name")
	fs.StringVar(&c.S3Region, "s3-region", envOr("LFSD_S3_REGION", "us-east-1"), "S3 region (any non-empty value for MinIO)")
	fs.StringVar(&c.S3Endpoint, "s3-endpoint", envOr("LFSD_S3_ENDPOINT", ""), "custom S3 endpoint for MinIO/R2; empty for AWS")
	fs.StringVar(&c.S3AccessKeyID, "s3-access-key-id", envOr("LFSD_S3_ACCESS_KEY_ID", ""), "S3 access key id")
	fs.StringVar(&c.S3SecretAccessKey, "s3-secret-access-key", envOr("LFSD_S3_SECRET_ACCESS_KEY", ""), "S3 secret access key")
	fs.BoolVar(&c.S3UsePathStyle, "s3-use-path-style", envBool("LFSD_S3_USE_PATH_STYLE", false), "use path-style addressing (required for MinIO)")
	fs.StringVar(&c.S3Prefix, "s3-prefix", envOr("LFSD_S3_PREFIX", ""), "optional key prefix for stored objects")
	fs.DurationVar(&c.URLExpiry, "url-expiry", envDur("LFSD_URL_EXPIRY", 10*time.Minute), "how long minted transfer URLs remain valid")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return c, nil
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
