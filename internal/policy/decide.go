package policy

// Decide is the Policy Decision Point: a pure function over a Policy and a
// Request. It walks the subject's principals in order and returns the first
// permit; absence of any matching grant is a denial (allow-only,
// first-match-wins). It performs no I/O, reads no globals, and never panics
// on input. See docs/auth-design.md §5.
func Decide(p *Policy, req Request) Decision {
	rp, ok := p.Repos[req.Resource.Repo]
	if !ok {
		// Ungoverned repos are short-circuited by the PEP before reaching us.
		// If we are called anyway, fail closed.
		return Decision{Effect: Deny, Reason: "repo not in policy"}
	}
	for _, name := range req.Subject.Principals {
		pg := rp.Principals[name]
		if pg == nil {
			continue
		}
		tr := pg.Tries[req.Action]
		if tr == nil {
			continue
		}
		if id, ok := tr.Decide(req.Resource.Path); ok {
			return Decision{
				Effect:      Permit,
				Source:      name,
				MatchedRule: id,
				Reason:      "permitted by " + name,
			}
		}
	}
	return Decision{Effect: Deny, Reason: "no grant matched any principal"}
}

// DecideAllPaths authorizes an action over a set of paths under all-paths
// semantics: it returns Permit only when Decide permits every path for the
// subject. The first denied path short-circuits and is returned alongside its
// Deny decision so the caller can attribute the refusal in a 403/audit line.
// An empty path set fails closed. It composes Decide and is likewise pure:
// no I/O, no globals, never panics on input. Used to gate reads of a redacted
// commit message bound to every path its commit touched (all-paths
// visibility). See docs/auth-design.md §5.
func DecideAllPaths(p *Policy, subj Subject, action Action, repo string, paths []string) (decision Decision, deniedPath string) {
	if len(paths) == 0 {
		return Decision{Effect: Deny, Reason: "no paths bound"}, ""
	}
	for _, path := range paths {
		d := Decide(p, Request{
			Subject:  subj,
			Action:   action,
			Resource: Resource{Repo: repo, Path: path},
		})
		if d.Effect != Permit {
			return d, path
		}
	}
	return Decision{Effect: Permit, Reason: "permitted on all bound paths"}, ""
}
