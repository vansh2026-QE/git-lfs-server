// Package ports declares the interfaces every consumer (PEP, Loader, Verifier)
// depends on. Implementations live in internal/memstore (in-memory) and
// adapters/* (production backends, deferred). No file in this package imports
// anything outside internal/policy. See docs/auth-design.md §4.
package ports
