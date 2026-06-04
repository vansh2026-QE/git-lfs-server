package ports

// MessageStore records redacted commit messages together with the set of paths
// their commit touched, and serves the message bytes back to authorized
// readers. The modified git client redacts the real message at commit time
// (replacing it with a "msg:<oid>" placeholder in the commit object) and Puts
// the bytes here at push time; the PEP reads BoundPaths to apply all-paths
// visibility (policy.DecideAllPaths) before streaming Message bytes to the
// reviewer extension.
//
// The OID is content-addressed (sha256 of the message bytes), mirroring LFS
// objects. The bound path set is supplied by the writer (the commit's staged
// paths) and is authoritative: a reader supplies only the OID and never
// influences which paths gate the read, so it cannot narrow the set to bypass
// all-paths semantics (cf. SF-1 in docs/security-findings.md).
//
// Contract:
//   - Put stores the message bytes for (repo, oid) and binds them to paths.
//     Idempotent on (repo, oid): re-Putting overwrites the bound path set and
//     bytes (content-addressing makes the bytes identical anyway).
//   - BoundPaths returns the paths bound to (repo, oid); an unknown OID
//     returns a nil slice with a nil error.
//   - Message returns the stored bytes for (repo, oid); an unknown OID returns
//     nil bytes with a nil error ("not found" is not an error — the caller
//     maps it to 404).
//   - Errors are reserved for infrastructure failures (disk, DB, I/O).
type MessageStore interface {
	Put(repo, oid string, paths []string, message []byte) error
	BoundPaths(repo, oid string) ([]string, error)
	Message(repo, oid string) ([]byte, error)
}
