# GitLab-backed workflow (PAT auth, pointers on GitLab, bytes in lfsd)

This is the end-to-end runbook for using `lfsd` with the **GitLab** auth backend:
GitLab hosts the repository and the LFS *pointers*, while `lfsd` stores the
actual object *bytes* and authorizes every transfer against the per-path policy.

> **Run `scripts/lfs-setup.sh` first** to wire the working copy (forked
> `git-lfs`, `lfs.url`, credentials). Everything below assumes you start there.

## How the pieces fit (read this first)

A single `git push` talks to **two different servers**, each with its own login:

```
git push origin main
   ├── commits + LFS pointers ──► GitLab          (auth: PAT, read_repository/write_repository)
   └── LFS object bytes ────────► lfsd             (auth: PAT, read_user, resolved to user:<you>)
```

- `lfs.url` (in `.git/config`) decides where the **bytes** go — point it at `lfsd`.
- The `origin` remote decides where **refs/pointers** go — point it at GitLab.
- These credentials are **separate logins**. One PAT can serve both *only if it
  carries all the scopes*: `read_user` (for `lfsd`) plus
  `read_repository`/`write_repository` (for the GitLab git push).

Because GitLab is itself an LFS server, it runs an LFS **integrity check** on
push and rejects pointers whose bytes it cannot find in its own store (yours are
in `lfsd`). You therefore **disable Git LFS on the GitLab project** (step 5) so
GitLab stores the pointers as plain files instead of validating them.

## Prerequisites

- Go 1.24+.
- A GitLab Personal Access Token with **`read_user` + `read_repository` +
  `write_repository`** scopes. Verify it:

```bash
export TOKEN=glpat-xxxxxxxxxxxx
export GL=https://gitlab.com            # or your self-managed instance
curl -s -H "Authorization: Bearer $TOKEN" "$GL/api/v4/user" | grep -o '"username":"[^"]*"'
```

  The username it prints is your policy principal. The policy in
  [examples/policy.json](examples/policy.json) must grant `user:<that-username>`
  (e.g. `user:vansh2026` is already present granting `public/**` and
  `private/privVansh/**`).

- The **forked** `git-lfs` client built (it sends the object `name` the policy
  server needs):

```bash
( cd client-implementation/git-lfs && go build -o bin/git-lfs . )
```

## Step 1 — Configure and start lfsd (the config file)

`scripts/run-lfsd.sh` reads `lfsd.env` at the repo root. **Create `lfsd.env` and
add your settings** (this is the "config file" — fill in your GitLab URL):

```bash
# lfsd.env
LFSD_AUTH_BACKEND=gitlab
LFSD_GITLAB_BASE_URL=https://gitlab.com     # <-- your GitLab instance
LFSD_AUTH_CACHE_TTL=5m
LFSD_AUTH_CACHE_NEGATIVE_TTL=30s
LFSD_STORAGE_BACKEND=local
LFSD_BASE_URL=http://localhost:8080
```

```bash
./scripts/run-lfsd.sh        # builds lfsd and runs it with lfsd.env
```

`lfsd` itself holds no credential: it validates each user's token against
GitLab at request time. Leave it running; it logs every decision on stderr.

## Step 2 — Wire the working copy with `scripts/lfs-setup.sh` (run this first)

This bootstraps a repo that LFS-tracks everything, points `lfs.url` at `lfsd`,
sets the `403`-as-pointer behavior, and adds GitLab as `origin`. **Add your
credentials here**: the Basic *username* is ignored by `lfsd` (use `oauth2`),
and the *password* is your PAT.

```bash
./scripts/lfs-setup.sh init \
  -u oauth2 -p "$TOKEN" \
  -r demo \
  -o "https://oauth2:$TOKEN@gitlab.com/<you>/<project>.git" \
  -n "Your Name" -e "you@example.com" \
  gitlab-lfs-work
cd gitlab-lfs-work
```

Notes:
- `-r demo` is the policy repo key (the `/demo` path in `lfs.url`); keep it
  matching [examples/policy.json](examples/policy.json).
- `-o ...` sets `origin` to your GitLab project, with the PAT embedded so git
  does not prompt. This writes credentials into `.git/config` (fine for local
  use; do not commit or share it).
- After this, `git config --get lfs.url` should read
  `http://oauth2:<token>@localhost:8080/demo`.

## Step 3 — Add files under granted paths and commit

Only paths your policy grants will upload; anything else is denied (the push is
all-or-nothing):

```bash
mkdir -p public private/privVansh
echo "hello world"   > public/hello.txt
echo "private stuff" > private/privVansh/v.txt
git add public private
git commit -m "add files"
```

## Step 4 — Disable Git LFS on the GitLab project

In the GitLab UI: **Project → Settings → General → Visibility, project
features, permissions → toggle Git LFS off → Save changes.**

This turns off GitLab's LFS integrity check so it accepts your pointers (whose
bytes live in `lfsd`) and shows them as plain text. Skipping this gives the
`remote rejected ... LFS objects are missing` error in step 5.

## Step 5 — Push

```bash
git push origin main
```

Expected: `git-lfs` uploads the bytes to `lfsd` (its log shows
`effect=permit source=user:<you>`), and GitLab accepts the refs and pointers.
Browse the project on GitLab — your files show their pointer text
(`version … / oid sha256:… / size …`).

## Credentials & where they go (quick reference)

| What | Where you put it | Scope needed |
| --- | --- | --- |
| lfsd server settings | `lfsd.env` (`LFSD_AUTH_BACKEND=gitlab`, `LFSD_GITLAB_BASE_URL=…`) | n/a |
| LFS object transfer (to lfsd) | `lfs.url` in `.git/config`, e.g. `http://oauth2:<PAT>@localhost:8080/demo` | `read_user` |
| Git push/clone (to GitLab) | `origin` remote URL, e.g. `https://oauth2:<PAT>@gitlab.com/<you>/<proj>.git` | `read_repository`, `write_repository` |

The PAT in `lfs.url` and the PAT in the `origin` URL are independent logins;
they only happen to be the same token when it carries all three scopes.

## Troubleshooting (errors seen during bring-up)

- **`bind: address already in use`** — a previous `lfsd` is still on the port.
  `ss -ltnp | grep :8080` then `kill <pid>`, or run on another port with
  `LFSD_ADDR=:8081` + `LFSD_BASE_URL=http://localhost:8081` in `lfsd.env`.
- **`HTTP Basic: Access denied` (on git push)** — GitLab git auth. You used an
  account password or a PAT without `write_repository`. Use a correctly-scoped
  PAT as the password.
- **`batch response: Authentication required: unauthorized`** — the LFS
  transfer was rejected. Check `git config --get lfs.url`: if it is empty or
  points at GitLab, set it to `lfsd`; if it points at `lfsd`, the token is
  expired/missing `read_user`, the `LFSD_GITLAB_BASE_URL` is wrong, or you are
  inside the 30s negative-cache window (wait or restart `lfsd`).
- **`remote rejected ... GitLab: LFS objects are missing`** — GitLab's LFS
  integrity check. Do step 4 (disable Git LFS on the project). Do **not** run
  `git lfs push --all` to satisfy it — that copies bytes into GitLab and
  bypasses `lfsd`'s policy.
- **Per-object `403` / files stay pointers** — authorization, not
  authentication: your username does not match a policy principal, or the path
  is not granted. Confirm the username from the `curl …/api/v4/user` check
  equals the `user:<…>` key in [examples/policy.json](examples/policy.json),
  and restart `lfsd` after editing the policy (it is read once at startup).

## Notes

- The in-memory path index is forgotten on `lfsd` restart; re-push before
  re-cloning after a restart.
- Plaintext HTTP and embedded tokens are for local testing only. Front `lfsd`
  with TLS in any real deployment. See [docs/auth-design.md](docs/auth-design.md)
  §4.1 and [docs/security-findings.md](docs/security-findings.md) SF-2.
