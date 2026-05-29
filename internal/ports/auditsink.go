package ports

import "time"

// AuditSink records authorisation decisions for post-hoc inspection. It is
// fire-and-forget: the PEP never waits on it. Implementations own their own
// buffering, back-pressure, and error handling; Record must never block and
// must never return an error.
//
// See docs/auth-design.md §4.4.
type AuditSink interface {
	Record(entry AuditEntry)
}

// AuditEntry is the per-decision record written by the PEP. Fields are
// deliberately denormalised so audit storage can be append-only without
// joining to a live policy.
//
// Effect and Source mirror policy.Decision but are stored here as strings
// to keep AuditSink free of any policy-package import; production sinks
//  need to depend only on this package.
// See docs/auth-design.md §4.4 and §7.4.
type AuditEntry struct {
	Timestamp time.Time
	RequestID string
	Subject   string // principal that authenticated, e.g. "user:alice"
	Action    string
	Repo      string
	Path      string
	OID       string
	Effect    string // "permit" or "deny"
	Source    string // matched principal; "" on deny
	Reason    string
}
