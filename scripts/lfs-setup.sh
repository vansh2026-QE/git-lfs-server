#!/usr/bin/env bash
# scripts/lfs-setup.sh — set up a working copy wired to the lfsd policy server
# using the forked git-lfs. Two modes:
#   init   bootstrap a NEW repo that tracks everything (except git metadata)
#          via LFS, set a local git identity, and commit the .gitattributes.
#   clone  clone an EXISTING repo (assumed to already carry that .gitattributes)
#          and just wire the client (endpoint, creds, 403-as-declined).
set -euo pipefail

SERVER="http://localhost:8080"; REPO="demo"; USER=""; PASSWORD=""; ORIGIN=""
NAME=""; EMAIL=""

usage() {
  cat <<'EOF'
Usage:
  lfs-setup.sh init  -u USER [-s SERVER] [-r REPO] [-p PASSWORD] [-o ORIGIN] [-n NAME] [-e EMAIL] DIR
  lfs-setup.sh clone -u USER [-s SERVER] [-r REPO] [-p PASSWORD] [-n NAME] [-e EMAIL] REMOTE [DIR]

  -u USER       LFS user (e.g. alice, bob). Required.
  -s SERVER     lfsd origin (default: http://localhost:8080)
  -r REPO       policy repo name in the LFS URL path (default: demo)
  -p PASSWORD   user password (default: <USER>pw)
  -o ORIGIN     (init only) git remote to add as origin
  -n NAME       local git user.name  (init: prompted if omitted)
  -e EMAIL      local git user.email (init: prompted if omitted)
EOF
}

use_fork_lfs() {
  local here bin
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  bin="$here/../client-implementation/git-lfs/bin"
  [ -x "$bin/git-lfs" ] && export PATH="$bin:$PATH"
  echo "git-lfs: $(command -v git-lfs) ($(git-lfs version))"
}

write_gitattributes() {
  cat > .gitattributes <<'EOF'
* filter=lfs diff=lfs merge=lfs -text
.gitattributes !filter !diff !merge text
.gitignore !filter !diff !merge text
EOF
}

lfs_url_auth() {
  printf '%s' "${SERVER%/}/$REPO" | sed -E "s#^([a-z]+)://#\1://$USER:$PASSWORD@#"
}

# Set the repo-local git identity, prompting for anything still unset so the
# alice/bob commits are attributable. Defaults derive from -u USER.
set_identity() {
  [ -n "$NAME" ]  || read -rp "Git user.name [$USER]: " NAME || true
  NAME="${NAME:-$USER}"
  [ -n "$EMAIL" ] || read -rp "Git user.email [$USER@example.com]: " EMAIL || true
  EMAIL="${EMAIL:-$USER@example.com}"
  git config --local user.name "$NAME"
  git config --local user.email "$EMAIL"
  echo "Local identity: $NAME <$EMAIL>"
}

cmd="${1:-}"; shift || true
[ "$cmd" = init ] || [ "$cmd" = clone ] || { usage; exit 2; }

while getopts "u:s:r:p:o:n:e:h" opt; do case "$opt" in
  u) USER="$OPTARG";; s) SERVER="$OPTARG";; r) REPO="$OPTARG";;
  p) PASSWORD="$OPTARG";; o) ORIGIN="$OPTARG";; n) NAME="$OPTARG";;
  e) EMAIL="$OPTARG";; h) usage; exit 0;; *) usage; exit 2;;
esac; done
shift $((OPTIND - 1))
[ -n "$USER" ] || { echo "error: -u USER required" >&2; usage; exit 2; }
PASSWORD="${PASSWORD:-${USER}pw}"

use_fork_lfs

case "$cmd" in
  init)
    [ $# -ge 1 ] || { echo "error: DIR required" >&2; usage; exit 2; }
    git init -b main "$1"; cd "$1"
    [ -n "$ORIGIN" ] && git remote add origin "$ORIGIN"
    git config lfs.url "$(lfs_url_auth)"
    git config lfs.skipdownloaderrorcodes 403
    git lfs install --local >/dev/null
    set_identity
    write_gitattributes
    git add .gitattributes
    git commit -m "Track all files with Git LFS (except git metadata)" >/dev/null
    echo "Initialized $(pwd): all files except .gitattributes/.gitignore are LFS-tracked."
    ;;
  clone)
    [ $# -ge 1 ] || { echo "error: REMOTE required" >&2; usage; exit 2; }
    REMOTE="$1"; DIR="${2:-}"
    git clone -c lfs.url="$(lfs_url_auth)" -c lfs.skipdownloaderrorcodes=403 \
      "$REMOTE" ${DIR:+"$DIR"}
    cd "${DIR:-$(basename "${REMOTE%.git}")}"
    git lfs install --local >/dev/null
    { [ -n "$NAME" ] || [ -n "$EMAIL" ]; } && set_identity
    echo "Cloned into $(pwd), wired to $(git config lfs.url | sed -E 's#://[^@]*@#://#')."
    ;;
esac
