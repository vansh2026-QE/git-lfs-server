# Reviewer Extension Integration

This documents the **server-side contract** that the GitLab reviewer browser
extension uses to dereference LFS pointers (view files/diffs) against `lfsd`.
The extension itself lives in a separate, private/sideloaded repo and is out of
scope here; this file is the interface it codes against.

## Overview

The extension reads LFS pointers from a GitLab file/diff page (each pointer
carries an `oid`, `size`, and the file `path`), then asks `lfsd` for the bytes
behind each pointer and renders the file or diff client-side. `lfsd` validates
the reviewer's GitLab OAuth2 token and authorizes the read against the path
policy before streaming bytes.

```mermaid
sequenceDiagram
    participant Ext as Extension
    participant GL as GitLab (OAuth2 + repo)
    participant LFSD as lfsd
    Ext->>GL: OAuth2 auth-code + PKCE -> access token
    Ext->>GL: read pointer(s) from file/diff view (oid, size, path)
    Ext->>LFSD: GET /{repo}/content?oid=..&name=path (Authorization: Bearer token)
    LFSD->>GL: validate token (GET /api/v4/user, cached)
    LFSD->>LFSD: VerifyDownloadClaim + policy Decide(download, path)
    LFSD-->>Ext: 200 bytes (or 401/403/404)
    Ext->>Ext: render file; for diffs fetch base+head oids and diff client-side
```

## Content endpoint

```
GET /{repo}/content?oid=<oid>&name=<path>
Authorization: Bearer <gitlab-oauth2-or-pat-token>
```

- `{repo}` is the **lfsd policy repo key** (e.g. `demo`), *not* the GitLab
  project name. The extension supplies this key; lfsd does not map GitLab
  project names to policy repos.
- `oid` is the LFS object id from the pointer. Required (missing -> `400`).
- `name` is the file path claimed by the pointer; it is verified against the
  recorded `oid`->`path` binding and used for the policy decision.

Responses:

| Status | Meaning |
| ------ | ------- |
| `200`  | Streams the object bytes (`application/octet-stream`). |
| `400`  | Missing `oid`. |
| `401`  | Missing/invalid token (with `WWW-Authenticate` when the server runs with auth required). |
| `403`  | Token valid but the policy denies `download` on the path, or the `oid`/`name` claim does not match. |
| `404`  | No object recorded for that `oid` (or the blob is absent). |

The endpoint reuses the exact download authorization the LFS batch path uses, so
the `oid`->`path` binding protections (SF-1) still apply: a reviewer cannot read
a private object by claiming a different, readable path.

For a **diff**, the extension fetches the base and head blobs (two calls with the
two commits' pointer oids) and diffs the text itself; there is no server-side
differ.

## Commit-message endpoint

Redacted commit messages are hidden the same way as file contents: the commit
object on GitLab carries only a placeholder, and the real message is served by
`lfsd` behind the path policy. Unlike `/content`, the message store is
independent of the storage backend, so this endpoint is available on both the
`local` and `s3` deployments.

### Placeholder format

A redacted commit message is a single line in the commit object:

```
[redacted] msg:<oid>
```

- `<oid>` is the sha256 (64 hex chars) of the original message bytes.
- The extension scans commit/MR message text for `msg:([0-9a-f]{64})` and
  replaces the placeholder with the rehydrated message.

### Read (extension)

```
GET /{repo}/message?oid=<oid>
Authorization: Bearer <gitlab-oauth2-or-pat-token>
```

Authorized under **all-paths visibility**: the reader must be permitted to
`download` *every* path the commit touched. The reader supplies only the `oid`;
the bound path set is recorded server-side and cannot be narrowed, so the same
binding protection as `/content` applies (cf. SF-1).

| Status | Meaning |
| ------ | ------- |
| `200`  | The message bytes (`text/plain; charset=utf-8`). |
| `400`  | Missing `oid`. |
| `401`  | Missing/invalid token. |
| `403`  | Policy denies `download` on at least one bound path. |
| `404`  | No message recorded for that `oid`. |

### Record (client, not the extension)

```
POST /{repo}/message
Authorization: Bearer <token>
Content-Type: application/json

{ "oid": "<sha256>", "paths": ["<path>", ...], "message": "<real message>" }
```

Used by the client's `pre-push` hook (`contrib/lfs-msg-push.sh`), not the
extension. The caller must be permitted to `upload` *every* listed path, and
`oid` must equal `sha256(message)`; otherwise the record is rejected (`403`/`400`).

## OAuth2 (extension side)

The extension obtains the token via the GitLab OAuth2 **authorization-code flow
with PKCE** (no client secret in a public client):

- Register a GitLab OAuth application with redirect URI
  `https://<extension-id>.chromiumapp.org/` (Chrome `identity.launchWebAuthFlow`)
  or the Firefox equivalent.
- Request the scopes the policy needs to resolve the user (at minimum
  `read_user`; lfsd resolves the username via `GET /api/v4/user`).
- Send the resulting access token to lfsd as `Authorization: Bearer <token>`.

## CORS

Set the allowlist with `LFSD_CORS_ORIGINS` (comma-separated), e.g.:

```
LFSD_CORS_ORIGINS=chrome-extension://<id>,moz-extension://<uuid>
```

- Default is empty, which **disables** CORS.
- CORS is only required when the extension calls lfsd from a **content-script**
  context (the request carries the page origin and is subject to CORS).
- Fetches from a **background service worker** with `host_permissions` for the
  lfsd origin bypass CORS entirely, so the allowlist is unnecessary in that case
  (the recommended setup).
- Credentialed CORS is not enabled; the extension authenticates with a Bearer
  token, not cookies.

## Out of scope (server side)

- The extension itself: OAuth2 client, GitLab page parsing for `oid`/`path`,
  diff rendering.
- The S3/MinIO backend has no local BlobStore to stream from, so `/content` is
  served by the local backend only; an S3 deployment would need a proxy or a
  presigned-redirect variant.
- Mapping GitLab project names to lfsd repo keys (the extension supplies the
  key).
