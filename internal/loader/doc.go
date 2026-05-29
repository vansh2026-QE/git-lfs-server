// Package loader consumes raw policy bytes via ports.PolicyStore and produces
// a validated, in-memory PolicyModel. It owns JSON decoding, version checks,
// schema validation, path normalization, and trie construction.
// See docs/auth-design.md §10.
package loader
