package pep

import (
	"io"
	"net/http"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// LocalServer is the demo HTTP boundary: the batch Server plus the open byte
// transfer endpoints (PUT/GET /{repo}/objects/{oid}). Production uses the bare
// Server because S3/MinIO serves bytes directly, so the BlobStore dependency
// and these endpoints live only here. Both types satisfy http.Handler, so the
// caller wires whichever the deployment needs.
type LocalServer struct {
	*Server
	blobs ports.BlobStore
	mux   *http.ServeMux
}

// NewLocalServer wraps a batch Server with the local transfer endpoints backed
// by blobs. The mux re-registers the batch route alongside the transfer routes
// so the composed handler serves the whole demo surface.
func NewLocalServer(s *Server, blobs ports.BlobStore) *LocalServer {
	ls := &LocalServer{Server: s, blobs: blobs}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{repo}/objects/batch", s.batchHandler)
	mux.HandleFunc("PUT /{repo}/objects/{oid}", ls.putObject)
	mux.HandleFunc("GET /{repo}/objects/{oid}", ls.getObject)
	mux.HandleFunc("GET /{repo}/content", ls.getContent)
	ls.mux = mux
	return ls
}

func (ls *LocalServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ls.mux.ServeHTTP(w, r)
}

// putObject is the open PUT transfer endpoint. The batch handler already
// authorized this upload and minted the capability URL, so the byte transfer
// itself carries no auth; it streams the request body into the BlobStore.
func (ls *LocalServer) putObject(w http.ResponseWriter, r *http.Request) {
	if err := ls.blobs.Put(r.PathValue("repo"), r.PathValue("oid"), r.Body); err != nil {
		writeError(w, http.StatusInternalServerError, "store object")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// getObject is the open GET transfer endpoint, the download counterpart to
// putObject. A missing object is a 404; otherwise the bytes are streamed back.
func (ls *LocalServer) getObject(w http.ResponseWriter, r *http.Request) {
	repo, oid := r.PathValue("repo"), r.PathValue("oid")
	switch ok, err := ls.blobs.Exists(repo, oid); {
	case err != nil:
		writeError(w, http.StatusInternalServerError, "stat object")
		return
	case !ok:
		writeError(w, http.StatusNotFound, "object not found")
		return
	}
	rc, err := ls.blobs.Open(repo, oid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open object")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, rc)
}

// getContent is the authenticated, policy-checked read path the reviewer
// extension uses to dereference an LFS pointer. Unlike getObject (an open
// capability URL reached only after a batch authorization), this endpoint runs
// the full gate itself: authenticate the GitLab token, verify the claimed
// (oid, name) against the PathIndex, and authorize the download against the
// policy before streaming bytes. See docs/auth-design.md §4.1.1.
func (ls *LocalServer) getContent(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	subject, err := ls.auth.Authenticate(r)
	if err != nil {
		ls.writeUnauthorized(w)
		return
	}
	if ls.requireAuth && isAnonymous(subject) {
		ls.writeUnauthorized(w)
		return
	}

	oid := r.URL.Query().Get("oid")
	name := r.URL.Query().Get("name")
	if oid == "" {
		writeError(w, http.StatusBadRequest, "missing oid")
		return
	}

	path, status, verr := VerifyDownloadClaim(ls.index, repo, oid, name)
	if verr != nil {
		ls.recordAudit(r, subject, repo, OpDownload, objectOutcome{OID: oid, Status: status, VerifyErr: verr})
		writeError(w, status, verr.Error())
		return
	}

	dec := policy.Decide(ls.policy.Load(), policy.Request{
		Subject:  subject,
		Action:   policy.ActionDownload,
		Resource: policy.Resource{Repo: repo, Path: path},
	})
	ls.recordAudit(r, subject, repo, OpDownload, objectOutcome{OID: oid, Path: path, Decision: dec})
	if dec.Effect != policy.Permit {
		writeError(w, http.StatusForbidden, "download denied")
		return
	}

	switch ok, err := ls.blobs.Exists(repo, oid); {
	case err != nil:
		writeError(w, http.StatusInternalServerError, "stat object")
		return
	case !ok:
		writeError(w, http.StatusNotFound, "object not found")
		return
	}
	rc, err := ls.blobs.Open(repo, oid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open object")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, rc)
}

var _ http.Handler = (*LocalServer)(nil)
