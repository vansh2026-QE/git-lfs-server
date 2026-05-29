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
