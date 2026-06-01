package ports

import "io"

// BlobStore stores and serves raw object bytes for the local/demo deployment.
// It is the byte-transfer backend behind the open PUT/GET transfer endpoints:
// the PUT handler writes via Put, the GET handler reads via Open after gating
// on Exists.
//
// This seam is deliberately separate from ObjectStore. ObjectStore mints the
// transfer URLs and is always the server's job (it mints pre-signed S3 URLs in
// production). BlobStore exists only because, in the local demo, our own
// process is also the storage backend. In an S3/MinIO deployment the client
// transfers bytes directly to object storage, so neither BlobStore nor the
// transfer endpoints exist. See docs/auth-design.md §4.5.
//
// Contract:
//   - Put stores the bytes for (repo, oid). Objects are content-addressed by
//     oid, so re-Putting the same oid overwrites with identical bytes and is
//     effectively idempotent.
//   - Open returns a reader over a stored object; the caller closes it.
//     Opening an object that does not exist returns a non-nil error — callers
//     gate on Exists first rather than relying on a sentinel.
//   - Exists reports presence. A missing object is (false, nil), not an error.
//   - Errors are otherwise reserved for infrastructure failures (disk, I/O).
type BlobStore interface {
	Put(repo, oid string, r io.Reader) error
	Open(repo, oid string) (io.ReadCloser, error)
	Exists(repo, oid string) (bool, error)
}
