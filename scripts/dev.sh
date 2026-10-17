#!/usr/bin/env bash
# Only clean up the two processes this invocation owns. Never use broad pkill.
set -euo pipefail
cd "$(dirname "$0")/.."
dev_root=$(mktemp -d "${TMPDIR:-/tmp}/outpost-dev.XXXXXX")
backend_pid=""
frontend_pid=""
cleanup() {
  trap - EXIT INT TERM
  if [[ -n "$frontend_pid" ]]; then kill "$frontend_pid" 2>/dev/null || true; wait "$frontend_pid" 2>/dev/null || true; fi
  if [[ -n "$backend_pid" ]]; then kill "$backend_pid" 2>/dev/null || true; wait "$backend_pid" 2>/dev/null || true; fi
  rm -rf "$dev_root"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"${GO:-go}" build -tags dev -o "$dev_root/outpost" ./cmd/outpost
# Override to keep a development DB between runs. No state is kept in the checkout.
dev_state_dir="${OUTPOST_DEV_STATE_DIR:-$dev_root/state}"
"$dev_root/outpost" --state-dir "$dev_state_dir" migrate
"$dev_root/outpost" --state-dir "$dev_state_dir" serve --dev &
backend_pid=$!
(cd web && exec node node_modules/vite/bin/vite.js --host 127.0.0.1 --strictPort) &
frontend_pid=$!
wait -n "$backend_pid" "$frontend_pid"
