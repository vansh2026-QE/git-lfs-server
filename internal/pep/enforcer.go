package pep

import (
	"fmt"
	"net/http"

	"github.com/vansh2026/git-lfs-server/internal/policy"
	"github.com/vansh2026/git-lfs-server/internal/ports"
)

// objectOutcome is the per-object state threaded from the Verify and Decide
// stages into the Enforcer. The handler (Commit 11) builds these; Path and
// Decision are valid only when VerifyErr is nil. See docs/auth-design.md §8.2.
type objectOutcome struct {
	OID       string
	Size      int64
	Path      string          // trusted path; "" when verification failed
	Status    int             // verification HTTP status (e.g. 404, 403)
	VerifyErr error           // non-nil when verification failed
	Decision  policy.Decision // valid only when VerifyErr == nil
}

// UploadRejection carries the paths that caused an all-or-nothing upload batch
// to be rejected. The handler renders it as a top-level 403. See §8.4.
type UploadRejection struct {
	Repo  string
	Paths []string
}

// EnforceDownload applies per-object download semantics (§8.4): the batch is
// always 200; permitted objects get a download action, while verification
// failures and denials become per-object errors. A mint failure aborts the
// whole request (caller maps the error to 500).
func EnforceDownload(store ports.ObjectStore, repo string, outcomes []objectOutcome) (*BatchResponse, error) {
	resp := &BatchResponse{Transfer: "basic", Objects: make([]*Transfer, 0, len(outcomes))}
	for _, o := range outcomes {
		t := &Transfer{OID: o.OID, Size: o.Size}
		switch {
		case o.VerifyErr != nil:
			t.Error = &ObjectError{Code: o.Status, Message: o.VerifyErr.Error()}
		case o.Decision.Effect != policy.Permit:
			t.Error = &ObjectError{Code: http.StatusForbidden, Message: o.Decision.Reason}
		default:
			act, err := store.MintDownload(repo, o.OID, o.Path)
			if err != nil {
				return nil, fmt.Errorf("pep: mint download for %s: %w", o.OID, err)
			}
			t.Actions = map[string]*Action{"download": toAction(act)}
		}
		resp.Objects = append(resp.Objects, t)
	}
	return resp, nil
}

// EnforceUpload applies all-or-nothing upload semantics (§8.4): if any object
// is denied the whole batch is rejected (rejection non-nil, no actions minted);
// otherwise every object gets an upload action. Callers verify names upstream,
// so outcomes here carry valid paths and decisions. A mint failure aborts the
// whole request (caller maps the error to 500).
func EnforceUpload(store ports.ObjectStore, repo string, outcomes []objectOutcome) (*BatchResponse, *UploadRejection, error) {
	var denied []string
	for _, o := range outcomes {
		if o.Decision.Effect != policy.Permit {
			denied = append(denied, o.Path)
		}
	}
	if len(denied) > 0 {
		return nil, &UploadRejection{Repo: repo, Paths: denied}, nil
	}
	resp := &BatchResponse{Transfer: "basic", Objects: make([]*Transfer, 0, len(outcomes))}
	for _, o := range outcomes {
		act, err := store.MintUpload(repo, o.OID, o.Path, o.Size)
		if err != nil {
			return nil, nil, fmt.Errorf("pep: mint upload for %s: %w", o.OID, err)
		}
		resp.Objects = append(resp.Objects, &Transfer{
			OID:     o.OID,
			Size:    o.Size,
			Actions: map[string]*Action{"upload": toAction(act)},
		})
	}
	return resp, nil, nil
}

func toAction(a ports.ObjectAction) *Action {
	return &Action{Href: a.Href, Header: a.Header, ExpiresIn: a.ExpiresIn}
}
