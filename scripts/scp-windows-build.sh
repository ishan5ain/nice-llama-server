#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

LOCAL_PATH="${1:-"$ROOT_DIR/dist/nice-llama-server-windows-amd64.exe"}"
REMOTE_HOST="${REMOTE_HOST:-}"
REMOTE_USER="${REMOTE_USER:-}"
REMOTE_PATH="${2:-${REMOTE_PATH:-~/nice-llama-server-windows-amd64.exe}}"

if [[ ! -f "$LOCAL_PATH" ]]; then
  echo "Build artifact not found: $LOCAL_PATH" >&2
  echo "Run scripts/build-windows.sh first, or pass the built .exe path as the first argument." >&2
  exit 1
fi

if [[ -z "$REMOTE_HOST" || -z "$REMOTE_USER" ]]; then
  echo "REMOTE_HOST and REMOTE_USER must be set either in the environment or in $ROOT_DIR/.env." >&2
  exit 1
fi

REMOTE_PATH_PS="${REMOTE_PATH//\'/\'\'}"

echo "Removing existing remote file, if present: ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}"
ssh "${REMOTE_USER}@${REMOTE_HOST}" \
  "powershell -NoProfile -Command \"\$target = \$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath('${REMOTE_PATH_PS}'); if (Test-Path -LiteralPath \$target) { Remove-Item -LiteralPath \$target -Force }\""

echo "Copying $LOCAL_PATH to ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}"
scp "$LOCAL_PATH" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}"
