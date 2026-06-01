package main

import (
	"flag"
	"os"
)

// Config holds the lfsd runtime settings, sourced from flags with environment
// variable fallbacks. The defaults run a self-contained demo server on
// localhost with on-disk storage under ./lfs-data.
type Config struct {
	Addr        string // TCP listen address, e.g. ":8080"
	BaseURL     string // public origin used to mint transfer hrefs
	StorageRoot string // directory the BlobStore writes objects under
	PolicyPath  string // path to the JSON policy document
}

// parseConfig builds a Config from args. Each flag falls back to an
// environment variable, then to a demo-friendly default.
func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("lfsd", flag.ContinueOnError)
	var c Config
	fs.StringVar(&c.Addr, "addr", envOr("LFSD_ADDR", ":8080"), "TCP listen address")
	fs.StringVar(&c.BaseURL, "base-url", envOr("LFSD_BASE_URL", "http://localhost:8080"), "public origin for minted transfer URLs")
	fs.StringVar(&c.StorageRoot, "storage", envOr("LFSD_STORAGE", "./lfs-data"), "directory for stored objects")
	fs.StringVar(&c.PolicyPath, "policy", envOr("LFSD_POLICY", "examples/policy.json"), "path to the JSON policy document")
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
