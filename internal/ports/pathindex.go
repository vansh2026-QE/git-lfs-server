package ports

// PathIndex records and queries the (repo, oid) -> {paths} binding that the
// Verifier consults on download. Uploads write into it after a successful
// /verify; downloads read from it to turn a client-claimed name into a
// trusted path.
//
// Contract:
//   - Record is idempotent under set semantics: recording the same
//     (repo, oid, path) twice is a no-op.
//   - PathsFor returns every path the OID has been recorded at within this
//     repo. Order is unspecified but stable across calls in-process.
//   - An unknown OID returns an empty slice with a nil error — "not found"
//     is not an error.
//   - Errors are reserved for infrastructure failures (disk, DB, network).
//
// See docs/auth-design.md §4.2 and §9.
type PathIndex interface {
	Record(repo, oid, path string) error
	PathsFor(repo, oid string) ([]string, error)
}
