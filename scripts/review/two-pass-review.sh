#!/usr/bin/env bash
set -euo pipefail

# Minimal wrapper for slopgate — delegate to shared orchestrator.
# This exists because slopgate predates the standardized review infra
# and lacks its own orchestrator.

REPO_ROOT="$(git rev-parse --show-toplevel)"
SHARED_ORCHESTRATOR="/srv/storage/shared/agent-toolkit/bin/two-pass-review.sh"

if [ ! -x "$SHARED_ORCHESTRATOR" ]; then
  echo "Error: shared orchestrator not found: $SHARED_ORCHESTRATOR" >&2
  exit 1
fi

exec bash "$SHARED_ORCHESTRATOR"
