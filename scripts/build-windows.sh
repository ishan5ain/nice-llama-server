#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_PATH="${1:-"$ROOT_DIR/dist/nice-llama-server-windows-amd64.exe"}"

mkdir -p "$(dirname "$OUT_PATH")"
rm -rf "$OUT_PATH"

echo "Building Windows binary to: $OUT_PATH"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -o "$OUT_PATH" ./cmd/nice-llama-server

file "$OUT_PATH"
