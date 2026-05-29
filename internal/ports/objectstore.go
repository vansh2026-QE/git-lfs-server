package ports

// ObjectAction is a transfer action the client follows to move bytes: a URL
// plus optional auth headers and expiry. Mirrors the LFS batch action object.
type ObjectAction struct {
	Href      string
	Header    map[string]string
	ExpiresIn int // seconds; 0 means unset
}

// ObjectStore mints transfer URLs for authorized objects. v1 is local
// filesystem; production is S3/MinIO. Full design deferred (see
// docs/auth-design.md §4.5); this is the seam the Enforcer depends on so it
// can be tested with a fake.
type ObjectStore interface {
	MintUpload(repo, oid, path string, size int64) (ObjectAction, error)
	MintDownload(repo, oid, path string) (ObjectAction, error)
}
