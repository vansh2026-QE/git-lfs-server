package pep

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Operation is the LFS batch operation.
type Operation string

const (
	OpDownload Operation = "download"
	OpUpload   Operation = "upload"
)

// BatchRequest is the decoded LFS batch request body. Optional protocol
// fields the client sends (transfers, ref, hash_algo) are accepted and
// ignored. See client-implementation/git-lfs/tq/api.go.
type BatchRequest struct {
	Operation Operation     `json:"operation"`
	Objects   []BatchObject `json:"objects"`
}

// BatchObject is one requested object. Name is the client-claimed path used
// for path-level authorization; the fork's client sends it on request objects
// (omitempty, so it may be absent). The Verifier checks it on download and
// accepts it as a claim on upload. See docs/auth-design.md §9.
type BatchObject struct {
	OID  string `json:"oid"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// ErrBadBatch marks a malformed batch request; the handler maps it to HTTP 400.
var ErrBadBatch = errors.New("pep: malformed batch request")

// ParseBatchRequest decodes and defensively validates an LFS batch request.
// Unknown optional fields are tolerated (the client sends transfers/ref/
// hash_algo); structural problems return ErrBadBatch.
func ParseBatchRequest(r *http.Request) (*BatchRequest, error) {
	var b BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadBatch, err)
	}
	if b.Operation != OpDownload && b.Operation != OpUpload {
		return nil, fmt.Errorf("%w: bad operation %q", ErrBadBatch, b.Operation)
	}
	if len(b.Objects) == 0 {
		return nil, fmt.Errorf("%w: no objects", ErrBadBatch)
	}
	for i, o := range b.Objects {
		if o.OID == "" {
			return nil, fmt.Errorf("%w: object %d missing oid", ErrBadBatch, i)
		}
	}
	return &b, nil
}
