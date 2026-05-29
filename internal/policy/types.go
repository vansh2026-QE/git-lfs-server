package policy

// Subject is the acting identity for a request, expressed as a flat list of
// principals (user, groups, service accounts) it acts as. Membership is
// resolved by the PEP/identity layer before the PDP runs.
type Subject struct {
	// Principals walks most-specific first so audit attribution prefers the
	// user's own grant over inherited group grants.
	Principals []string
}

// Principal ID conventions. Use the matching
// prefix when building principal IDs; "anonymous" is the only reserved
// literal that carries no prefix.
const (
	PrincipalAnonymous     = "anonymous"
	PrincipalPrefixUser    = "user:"
	PrincipalPrefixGroup   = "group:"
	PrincipalPrefixService = "service:"
)

// Action names what the subject is trying to do. v1 supports upload and
// download; later additions are validated at load time, not decide time.
// See docs/auth-design.md §5.2.
type Action string

const (
	ActionUpload   Action = "upload"
	ActionDownload Action = "download"
)

// Resource identifies what is being acted on: a path inside a repository.
// Resource.Ref (refs as an authorization dimension) is reserved for the
// future and intentionally absent in v1. See docs/auth-design.md §5.6.
type Resource struct {
	Repo string
	Path string
}

// Request is the PDP input: who, doing what, to what.
type Request struct {
	Subject  Subject
	Action   Action
	Resource Resource
}

// Effect is the outcome of a Decision. Semantics are allow-only,
// first-match-wins: Permit when any principal's trie matches; Deny when
// none does. See docs/auth-design.md §2.1 principle 4.
type Effect int

// Deny is the zero value so that an uninitialized Decision fails closed.
// See docs/auth-design.md §2.1 principle 3.
const (
	Deny Effect = iota
	Permit
)

// String returns "permit" or "deny" for audit log rendering.
func (e Effect) String() string {
	switch e {
	case Permit:
		return "permit"
	case Deny:
		return "deny"
	default:
		return "unknown"
	}
}

// Decision is the PDP output. Source attributes the match to the principal
// whose trie matched (empty on Deny). MatchedRule is the stable grant ID
// assigned by the Loader. See docs/auth-design.md §5.2.
type Decision struct {
	Effect      Effect
	Source      string
	MatchedRule string
	Reason      string
}
