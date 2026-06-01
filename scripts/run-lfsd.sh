#!/usr/bin/env bash
# scripts/run-lfsd.sh — load settings from an env file and run lfsd, so you
# don't have to pass LFSD_* variables on the command line.
#
# Usage: scripts/run-lfsd.sh [ENV_FILE]   (default: lfsd.env at the repo root)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${1:-lfsd.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "run-lfsd: env file '$ENV_FILE' not found." >&2
  echo "          create it with your LFSD_* settings (see docs/demo.md)." >&2
  exit 1
fi

# Export every KEY=VALUE assignment in the env file into the environment so
# lfsd's flag fallbacks pick them up.
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

go build -o lfsd ./cmd/lfsd
exec ./lfsd
