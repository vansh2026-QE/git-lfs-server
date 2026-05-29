package pep

import "github.com/vansh2026/git-lfs-server/internal/policy"

// Verified is a batch object whose path the Verifier has established as trusted.
type Verified struct {
	OID  string
	Size int64
	Path string
}

// actionForOperation maps a batch operation to the policy action it requires.
func actionForOperation(op Operation) policy.Action {
	if op == OpUpload {
		return policy.ActionUpload
	}
	return policy.ActionDownload
}

// DecideAll authorizes every verified object against the policy and returns one
// Decision per input, in order. Denials are included: the audit log wants the
// full set, not just permits. See docs/auth-design.md §8.2 (job 4).
func DecideAll(p *policy.Policy, subject policy.Subject, repo string, op Operation, objs []Verified) []policy.Decision {
	action := actionForOperation(op)
	decisions := make([]policy.Decision, len(objs))
	for i, o := range objs {
		decisions[i] = policy.Decide(p, policy.Request{
			Subject:  subject,
			Action:   action,
			Resource: policy.Resource{Repo: repo, Path: o.Path},
		})
	}
	return decisions
}
