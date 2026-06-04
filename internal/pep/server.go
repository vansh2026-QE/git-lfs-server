package pep

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// lfsContentType is the media type the LFS Batch API speaks. The client both
// sends and requires it on the batch exchange.
const lfsContentType = "application/vnd.git-lfs+json"

// Server is the HTTP boundary for the LFS Batch API. It wires the ports
// together and holds the active policy snapshot. See docs/auth-design.md §8.
type Server struct {
	auth     ports.Authenticator
	index    ports.PathIndex
	store    ports.ObjectStore
	msgStore ports.MessageStore
	audit    ports.AuditSink
	policy   atomic.Pointer[policy.Policy]
	mux      *http.ServeMux

	// requireAuth, when set, rejects the anonymous subject with a 401 carrying
	// a WWW-Authenticate challenge so git-lfs invokes its credential helper.
	// Left false for the open demo backend. See docs/auth-design.md §7.
	requireAuth bool
}

// NewServer wires the adapters and an initial policy snapshot. The policy is
// loaded by the caller (main/tests via internal/loader) and passed in, keeping
// pep free of any loader import per the DIP layering. SetPolicy swaps the
// snapshot atomically for future hot reload.
func NewServer(auth ports.Authenticator, index ports.PathIndex, store ports.ObjectStore, audit ports.AuditSink, pol *policy.Policy) *Server {
	s := &Server{auth: auth, index: index, store: store, audit: audit}
	s.policy.Store(pol)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{repo}/objects/batch", s.batchHandler)
	mux.HandleFunc("GET /{repo}/message", s.handleMessageRead)
	mux.HandleFunc("POST /{repo}/message", s.handleMessageRecord)
	s.mux = mux
	return s
}

// SetPolicy atomically replaces the active policy snapshot.
func (s *Server) SetPolicy(pol *policy.Policy) { s.policy.Store(pol) }

// SetRequireAuth toggles mandatory authentication. When enabled, a request
// resolving to the anonymous subject is rejected with 401 + WWW-Authenticate
// instead of being run through the policy as anonymous.
func (s *Server) SetRequireAuth(v bool) { s.requireAuth = v }

// SetMessageStore enables the redacted commit-message endpoints. Left nil for
// deployments that don't use message hiding; the message routes then return 404.
func (s *Server) SetMessageStore(ms ports.MessageStore) { s.msgStore = ms }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// batchHandler implements POST /{repo}/objects/batch: authenticate -> parse ->
// per-object verify+decide -> enforce -> JSON. See docs/auth-design.md §8.2-§8.4.
func (s *Server) batchHandler(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	subject, err := s.auth.Authenticate(r)
	if err != nil {
		s.writeUnauthorized(w)
		return
	}
	if s.requireAuth && isAnonymous(subject) {
		s.writeUnauthorized(w)
		return
	}
	br, err := ParseBatchRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pol := s.policy.Load()

	outcomes := make([]objectOutcome, len(br.Objects))
	for i, obj := range br.Objects {
		outcomes[i] = s.evaluate(pol, subject, repo, br.Operation, obj)
		s.recordAudit(r, subject, repo, br.Operation, outcomes[i])
	}

	if br.Operation == OpUpload {
		s.respondUpload(w, repo, outcomes)
		return
	}
	s.respondDownload(w, repo, outcomes)
}

// evaluate verifies one object's claimed name and, if trusted, runs the PDP.
// policy.Decide is called per object (rather than DecideAll) so the verify
// outcome and decision stay threaded together in one objectOutcome, avoiding
// index realignment between the verified subset and the full object list.
func (s *Server) evaluate(pol *policy.Policy, subject policy.Subject, repo string, op Operation, obj BatchObject) objectOutcome {
	oc := objectOutcome{OID: obj.OID, Size: obj.Size}
	var (
		path   string
		status int
		verr   error
	)
	if op == OpUpload {
		path, status, verr = VerifyUploadClaim(obj.Name)
	} else {
		path, status, verr = VerifyDownloadClaim(s.index, repo, obj.OID, obj.Name)
	}
	if verr != nil {
		oc.Status, oc.VerifyErr = status, verr
		return oc
	}
	oc.Path = path
	oc.Decision = policy.Decide(pol, policy.Request{
		Subject:  subject,
		Action:   actionForOperation(op),
		Resource: policy.Resource{Repo: repo, Path: path},
	})
	return oc
}

// respondDownload writes the per-object 200 batch: permitted objects carry a
// download action, denials/verify-failures carry a per-object error.
func (s *Server) respondDownload(w http.ResponseWriter, repo string, outcomes []objectOutcome) {
	resp, err := EnforceDownload(s.store, repo, outcomes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	markAuthenticated(resp)
	writeJSON(w, http.StatusOK, resp)
}

// respondUpload applies all-or-nothing upload semantics and, on a fully
// permitted batch, records each (repo, oid, path) binding so later downloads
// can authorize against it (demo: recorded at batch time, no /verify).
func (s *Server) respondUpload(w http.ResponseWriter, repo string, outcomes []objectOutcome) {
	// An upload with no name is a malformed request (400), distinct from a
	// policy denial (403); surface it before the all-or-nothing decision.
	for _, o := range outcomes {
		if o.VerifyErr != nil {
			writeError(w, o.Status, o.VerifyErr.Error())
			return
		}
	}
	resp, rej, err := EnforceUpload(s.store, repo, outcomes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if rej != nil {
		writeError(w, http.StatusForbidden, "upload denied for: "+strings.Join(rej.Paths, ", "))
		return
	}
	for _, o := range outcomes {
		if rerr := s.index.Record(repo, o.OID, o.Path); rerr != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
	markAuthenticated(resp)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) recordAudit(r *http.Request, subject policy.Subject, repo string, op Operation, oc objectOutcome) {
	entry := ports.AuditEntry{
		Timestamp: time.Now().UTC(),
		RequestID: r.Header.Get("X-Request-Id"),
		Subject:   primaryPrincipal(subject),
		Action:    string(op),
		Repo:      repo,
		Path:      oc.Path,
		OID:       oc.OID,
	}
	if oc.VerifyErr != nil {
		entry.Effect = "deny"
		entry.Reason = oc.VerifyErr.Error()
	} else {
		entry.Effect = oc.Decision.Effect.String()
		entry.Source = oc.Decision.Source
		entry.Reason = oc.Decision.Reason
	}
	s.audit.Record(entry)
}

// handleMessageRead implements GET /{repo}/message?oid=<oid>: it authenticates
// the reader, looks up the path set the commit touched, and streams the real
// message only if the reader may download EVERY bound path (all-paths
// visibility via policy.DecideAllPaths). The reader supplies only the oid; the
// bound paths come from the store, so the read cannot be narrowed to bypass the
// gate (cf. SF-1). See docs/extension-integration.md.
func (s *Server) handleMessageRead(w http.ResponseWriter, r *http.Request) {
	if s.msgStore == nil {
		writeError(w, http.StatusNotFound, "message store not configured")
		return
	}
	repo := r.PathValue("repo")
	subject, err := s.auth.Authenticate(r)
	if err != nil {
		s.writeUnauthorized(w)
		return
	}
	if s.requireAuth && isAnonymous(subject) {
		s.writeUnauthorized(w)
		return
	}
	oid := r.URL.Query().Get("oid")
	if oid == "" {
		writeError(w, http.StatusBadRequest, "missing oid")
		return
	}
	paths, err := s.msgStore.BoundPaths(repo, oid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(paths) == 0 {
		writeError(w, http.StatusNotFound, "no message recorded for oid")
		return
	}
	decision, denied := policy.DecideAllPaths(s.policy.Load(), subject, policy.ActionDownload, repo, paths)
	s.recordMessageAudit(r, subject, "message-read", repo, oid, decision, denied)
	if decision.Effect != policy.Permit {
		writeError(w, http.StatusForbidden, "message hidden by policy")
		return
	}
	msg, err := s.msgStore.Message(repo, oid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if msg == nil {
		writeError(w, http.StatusNotFound, "no message recorded for oid")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(msg)
}

// recordMessageAudit logs a message-read decision, attributing a denial to the
// first bound path that failed the all-paths check.
func (s *Server) recordMessageAudit(r *http.Request, subject policy.Subject, action, repo, oid string, d policy.Decision, deniedPath string) {
	s.audit.Record(ports.AuditEntry{
		Timestamp: time.Now().UTC(),
		RequestID: r.Header.Get("X-Request-Id"),
		Subject:   primaryPrincipal(subject),
		Action:    action,
		Repo:      repo,
		Path:      deniedPath,
		OID:       oid,
		Effect:    d.Effect.String(),
		Source:    d.Source,
		Reason:    d.Reason,
	})
}

// messageRecordRequest is the pre-push payload that binds a redacted commit
// message to the paths its commit touched. OID must be the sha256 of Message.
type messageRecordRequest struct {
	OID     string   `json:"oid"`
	Paths   []string `json:"paths"`
	Message string   `json:"message"`
}

// handleMessageRecord implements POST /{repo}/message: the client uploads the
// real commit message (kept out of the commit object, which carries only the
// "msg:<oid>" placeholder) at push time. The caller must be permitted to upload
// EVERY bound path, and the oid must match the message digest, before the
// binding is stored. See docs/extension-integration.md.
func (s *Server) handleMessageRecord(w http.ResponseWriter, r *http.Request) {
	if s.msgStore == nil {
		writeError(w, http.StatusNotFound, "message store not configured")
		return
	}
	repo := r.PathValue("repo")
	subject, err := s.auth.Authenticate(r)
	if err != nil {
		s.writeUnauthorized(w)
		return
	}
	if s.requireAuth && isAnonymous(subject) {
		s.writeUnauthorized(w)
		return
	}
	var req messageRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.OID == "" || len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "oid and paths are required")
		return
	}
	sum := sha256.Sum256([]byte(req.Message))
	if hex.EncodeToString(sum[:]) != req.OID {
		writeError(w, http.StatusBadRequest, "oid does not match message digest")
		return
	}
	decision, denied := policy.DecideAllPaths(s.policy.Load(), subject, policy.ActionUpload, repo, req.Paths)
	s.recordMessageAudit(r, subject, "message-record", repo, req.OID, decision, denied)
	if decision.Effect != policy.Permit {
		writeError(w, http.StatusForbidden, "message record denied by policy")
		return
	}
	if err := s.msgStore.Put(repo, req.OID, req.Paths, []byte(req.Message)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"oid": req.OID})
}

func primaryPrincipal(sub policy.Subject) string {
	if len(sub.Principals) == 0 {
		return policy.PrincipalAnonymous
	}
	return sub.Principals[0]
}

// markAuthenticated flags permitted objects so the client treats their action
// hrefs as self-authorizing capability URLs (open transfer endpoints).
func markAuthenticated(resp *BatchResponse) {
	for _, t := range resp.Objects {
		if t.Actions != nil {
			t.Authenticated = true
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", lfsContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}

// writeUnauthorized returns 401. When auth is mandatory it adds a Basic
// challenge so git-lfs invokes its credential helper (CLI PAT delivered as the
// Basic password). See docs/auth-design.md §7.
func (s *Server) writeUnauthorized(w http.ResponseWriter) {
	if s.requireAuth {
		w.Header().Set("WWW-Authenticate", `Basic realm="Git LFS"`)
	}
	writeError(w, http.StatusUnauthorized, "unauthorized")
}

// isAnonymous reports whether a resolved Subject carries no real identity.
func isAnonymous(sub policy.Subject) bool {
	return len(sub.Principals) == 0 ||
		(len(sub.Principals) == 1 && sub.Principals[0] == policy.PrincipalAnonymous)
}

var _ http.Handler = (*Server)(nil)
