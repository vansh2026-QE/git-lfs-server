# Demo: policy-enforced Git LFS server

`lfsd` is a custom Git LFS API server. It authenticates the LFS batch request,
checks each object's path against a per-user policy, and only then mints
transfer URLs. This runbook walks the full permit/deny matrix using the forked
`git-lfs` client in `client-implementation/`.

## Prerequisites

- Go 1.22+
- The **forked** `git-lfs` client built and first on your `PATH`. It sends the
  object `name` in batch requests (upstream git-lfs does not), which is what
  lets the server authorize by path. Build it from
  `client-implementation/git-lfs` and confirm with `git lfs version`.

## 1. Start the server

```bash
go build -o lfsd ./cmd/lfsd
./lfsd        # :8080, policy examples/policy.json, storage ./lfs-data
```

Leave it running; it logs every decision via the stderr audit sink. Override
defaults with flags or `LFSD_*` env vars:

| flag | env | default | meaning |
|---|---|---|---|
| `-addr` | `LFSD_ADDR` | `:8080` | listen address |
| `-base-url` | `LFSD_BASE_URL` | `http://localhost:8080` | origin used in minted hrefs |
| `-storage` | `LFSD_STORAGE` | `./lfs-data` | object byte storage dir |
| `-policy` | `LFSD_POLICY` | `examples/policy.json` | policy document |

## 2. Policy (examples/policy.json)

| path | alice | bob |
|---|---|---|
| `public/**` | permit | permit |
| `private/privAlice/**` | permit | deny |
| `private/privBob/**` | deny | permit |

## 3. Set up a working repo

```bash
# Bare repo holds git refs; lfsd only handles LFS objects.
git init --bare /tmp/demo-origin.git

mkdir demo-work && cd demo-work
git init && git remote add origin /tmp/demo-origin.git
git lfs track "*.bin"
git add .gitattributes && git commit -m "track lfs"

# Point LFS at lfsd as alice (Basic auth via URL userinfo).
git config lfs.url http://alice:alicepw@localhost:8080/demo
```

## 4. alice pushes permitted paths -> 200

```bash
mkdir -p public private/privAlice
head -c 1024 /dev/urandom > public/hello.bin
head -c 1024 /dev/urandom > private/privAlice/secret.bin
git add public private && git commit -m "add objects"
git push origin main          # both objects upload; server logs effect=permit
```

## 5. alice is denied another user's space -> 403

```bash
mkdir -p private/privBob
head -c 1024 /dev/urandom > private/privBob/nope.bin
git add private/privBob && git commit -m "try privBob"
git push origin main          # batch rejected (all-or-nothing); push fails
```

## 6. bob pulls public (permit) but not alice's private (deny)

```bash
cd ..
git clone /tmp/demo-origin.git demo-clone && cd demo-clone
git config lfs.url http://bob:bobpw@localhost:8080/demo

git lfs pull --include "public/**"             # hello.bin downloads -> 200
git lfs pull --include "private/privAlice/**"  # per-object 403; object skipped
```

## Notes / demo caveats

- The transfer endpoints (`PUT`/`GET /{repo}/objects/{oid}`) are intentionally
  open. The batch response marks actions `authenticated: true` so the client
  does not resend credentials to them. Demo posture only.
- The path index is **in-memory**: restarting `lfsd` forgets which OIDs map to
  which paths, so re-push before re-pulling after a restart.
- Plaintext HTTP and hardcoded demo credentials — not for production. See the
  hardening backlog in `docs/auth-design.md`.
