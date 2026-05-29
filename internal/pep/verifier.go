package pep

import (
	"errors"
	"net/http"

	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// Verifier errors. Each maps to the HTTP status the helper returns alongside it.
var (
	// ErrUnknownOID: the OID has no recorded path in this repo (download 404).
	ErrUnknownOID = errors.New("pep: object not found")
	// ErrNameMismatch: the client's claimed name matches no recorded path (download 403).
	ErrNameMismatch = errors.New("pep: claimed name does not match recorded paths")
	// ErrEmptyName: an upload omitted the name that establishes the binding (upload 400).
	ErrEmptyName = errors.New("pep: upload requires a name")
)

// VerifyDownloadClaim turns a client-claimed (oid, name) into a trusted path
// by consulting the PathIndex. This is the gate that makes path-level
// authorization real: a client cannot be authorized for a path it lies about.
//
//   - infra failure   -> ("", 500, err)
//   - unrecorded oid  -> ("", 404, ErrUnknownOID)
//   - empty claim     -> (firstRecordedPath, 200, nil)
//   - matching claim  -> (claim, 200, nil)
//   - mismatched claim-> ("", 403, ErrNameMismatch)
//
// See docs/auth-design.md §9.4 and §9.6.
func VerifyDownloadClaim(idx ports.PathIndex, repo, oid, claimed string) (path string, status int, err error) {
	paths, err := idx.PathsFor(repo, oid)
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	if len(paths) == 0 {
		return "", http.StatusNotFound, ErrUnknownOID
	}
	if claimed == "" {
		// Multiple recorded paths with no claim is deterministic (PathsFor is
		// stable) but ambiguous; the caller may log this. See §9.5.
		return paths[0], http.StatusOK, nil
	}
	for _, p := range paths {
		if p == claimed {
			return claimed, http.StatusOK, nil
		}
	}
	return "", http.StatusForbidden, ErrNameMismatch
}

// VerifyUploadClaim accepts the client-claimed name as the binding to be
// established. Uploads do not consult the index (no binding exists until the
// upload completes); they only require that a name is present.
// See docs/auth-design.md §9.3 and §9.5.
func VerifyUploadClaim(claimed string) (path string, status int, err error) {
	if claimed == "" {
		return "", http.StatusBadRequest, ErrEmptyName
	}
	return claimed, http.StatusOK, nil
}
