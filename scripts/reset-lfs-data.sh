#!/usr/bin/env bash
# scripts/reset-lfs-data.sh — wipe lfsd's local object storage + path-index
# bindings so you can start a clean demo. Objects (lfs-data/<repo>/) and the
# matching index entry MUST be cleared together: a binding whose bytes are gone
# makes lfsd advertise objects it can't serve (downloads pass verification, then
# fail fetching the bytes).
#
# IMPORTANT: stop lfsd first. It holds the index in memory and rewrites the file
# on every upload, so changes made while it runs are overwritten. Needs python3.
set -euo pipefail

ROOT="${LFSD_STORAGE:-./lfs-data}"
INDEX="${LFSD_PATHINDEX:-$ROOT/pathindex.json}"
REPO="demo"; ALL=""; YES=""

usage() {
  cat <<EOF
Usage: reset-lfs-data.sh [-r REPO] [-a] [-y]
  -r REPO  repo whose objects + index entry to clear (default: demo)
  -a       reset ALL repos: remove every object dir and empty the index
  -y       skip the confirmation prompt

Env: LFSD_STORAGE    object root     (default ./lfs-data)
     LFSD_PATHINDEX  index json file (default \$LFSD_STORAGE/pathindex.json)
EOF
}

while getopts "r:ayh" opt; do case "$opt" in
  r) REPO="$OPTARG";; a) ALL=1;; y) YES=1;; h) usage; exit 0;; *) usage; exit 2;;
esac; done

if [ -n "$ALL" ]; then target="ALL repos under $ROOT + empty $INDEX"
else target="repo '$REPO' ($ROOT/$REPO + its entry in $INDEX)"; fi

echo "About to reset: $target"
echo "Make sure lfsd is STOPPED (it rewrites the index on upload)."
if [ -z "$YES" ]; then
  read -rp "Proceed? [y/N] " ans
  case "$ans" in y|Y) ;; *) echo "aborted"; exit 1;; esac
fi

if [ -n "$ALL" ]; then
  [ -d "$ROOT" ] && find "$ROOT" -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} +
  rm -f "$INDEX"
  echo "Cleared all object dirs under $ROOT and removed $INDEX."
else
  rm -rf "${ROOT:?}/$REPO"
  if [ -f "$INDEX" ]; then
    python3 - "$INDEX" "$REPO" <<'PY'
import json, os, sys
path, repo = sys.argv[1], sys.argv[2]
with open(path) as f:
    data = json.load(f)
data.pop(repo, None)
tmp = path + ".tmp"
with open(tmp, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
os.replace(tmp, path)
PY
  fi
  echo "Cleared $ROOT/$REPO and removed '$REPO' from $INDEX."
fi

echo "Done. Re-create a fresh demo with: scripts/lfs-setup.sh init -u <user> <dir>"
