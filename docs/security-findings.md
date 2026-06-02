# Security findings

## SF-1: Object laundering via upload claim (cross-path OID binding)

**Status:** known weakness, mitigation deferred (demo posture).

### What

Authorization is decided on the *path* an object is bound to in the `PathIndex`.
A user who knows an object's OID (e.g. from a pointer file they can read) can
bind that OID to a **different path they are allowed to write**, and thereby
gain read access to bytes they were never permitted to see.

### Scenario

1. alice uploads `private/privAlice/alice.txt` → index records
   `(demo, ALICE_OID, "private/privAlice/alice.txt")`. The bytes live in storage
   keyed by `ALICE_OID`.
2. bob can read his clone's *pointer* for that file (the smudge was denied, so it
   stays a pointer). The pointer reveals `ALICE_OID`.
3. bob sends an **upload** batch for `{oid: ALICE_OID, name: "public/evil.txt"}`.
   Policy permits bob on `public/**`, so the server records a *second* binding
   `(demo, ALICE_OID, "public/evil.txt")`.
4. bob now sends a **download** batch for the same `(oid, name)`. The claim
   matches a recorded path and policy permits `public/**`, so the server mints a
   download URL and bob receives **alice's bytes**.

### Why the git client does not trigger it

The stock/forked `git-lfs` only uploads objects it actually has in its local
store. bob never received `ALICE_OID` (he was denied), so the client has nothing
to upload and never sends the poisoning upload batch. A `git push` of the forged
pointer therefore moves only the tiny pointer blob; the binding is never created,
and a later clone fails `VerifyDownloadClaim` with a name mismatch (the file
stays a pointer). **The protection is incidental** — it relies on client
behaviour.

### Why it is still a real server-side hole

A hand-crafted request (e.g. `curl`) bypasses the client. The server records the
binding at **batch time, with no proof the caller possesses the bytes**
(`internal/pep/server.go`, the `index.Record` loop in `respondUpload`), and
`EnforceUpload` mints an action for any policy-permitted path without any
existence/possession check (`internal/pep/enforcer.go`). So the upload batch
alone poisons the index — no byte transfer required.

### Root cause

- Bindings are recorded at batch time without proof of possession.
- Storage is **content-addressed**: the bytes already exist under `ALICE_OID`, so
  knowing the hash is effectively a capability to the object.

### Why naive fixes are insufficient

- **`verify` via `HeadObject`** only confirms the object *exists* (it does,
  because alice uploaded it) — not that the claimant supplied it.
- **"skip upload if object exists"** is worse: it lets the claim succeed faster.

### Candidate mitigations (not yet implemented)

1. **Proof of possession** before recording a binding for an existing OID — e.g.
   challenge the client to return an HMAC over a server-chosen byte range, so
   only a holder of the actual bytes can bind it.
2. **First-writer-wins / per-principal binding** — a different principal claiming
   an OID already introduced by someone else does not auto-authorize.
3. **Deny + audit** any upload whose OID already exists under a different
   path/owner, rather than silently adding a binding.

## SF-2: GitLab token handling (`internal/gitlabauth`)

**Status:** documented posture for the GitLab `Authenticator`.

### Token handling

- **No raw tokens at rest in memory.** The validation cache is keyed by a
  SHA-256 digest of the token (`cache.go`, `cacheKey`); the raw PAT/OAuth2
  token is never used as a map key and never stored in a cache entry.
- **No tokens in logs.** Tokens are not logged on success or failure. Validator
  errors carry GitLab status codes, not credentials. Keep it that way when
  adding diagnostics.
- **Fail closed.** A rejected token (`401/403`) and any infrastructure failure
  reaching GitLab both resolve to HTTP 401; lfsd never falls back to anonymous
  on an *attempted* authentication.

### Residual considerations (accepted for now)

1. **Revocation lag.** A token revoked in GitLab stays usable until its cached
   entry expires (`-auth-cache-ttl`, default 5m). Lower the TTL where prompt
   revocation matters; the cost is more `GET /api/v4/user` round-trips.
2. **Negative-cache window.** A token rejected once is treated as invalid for
   `-auth-cache-negative-ttl` (default 30s) even if it becomes valid in that
   window; this throttles lookups for bad tokens at the cost of a short delay
   before a freshly valid token is accepted.
3. **Transport security.** Tokens travel as a Bearer header or Basic password;
   lfsd must be fronted by TLS in any non-loopback deployment so they are not
   exposed on the wire.
