// Package pep implements the Policy Enforcement Point: the HTTP boundary
// for the LFS Batch API. It contains the batch adapter, the OID-name
// Verifier, the Decider (which calls policy.Decide), and the Enforcer
// (which translates decisions into LFS-batch responses with per-direction
// atomicity). See docs/auth-design.md §8 and §9.
package pep
