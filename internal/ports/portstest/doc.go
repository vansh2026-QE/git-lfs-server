// Package portstest provides shared contract test suites for every
// implementation of an interface in internal/ports. In-memory adapters in
// internal/memstore and production adapters under adapters/* call these
// suites to prove they honour the contract identically.
// See docs/server-scaffold.md §5.2.
package portstest
