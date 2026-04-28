#!/usr/bin/env bash

set -euo pipefail

# Build a Linux ARM64 binary suitable for 64-bit Raspberry Pi OS / Raspbian.
# Usage: build-raspbian-arm64.sh [output_path]
#   output_path: optional, defaults to dist/nice-llama-server-raspbian-arm64

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<EOF
Usage: $(basename "$0") [output_path]
  output_path: optional, defaults to dist/nice-llama-server-raspbian-arm64
EOF
  exit 0
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_PATH="${1:-"$ROOT_DIR/dist/nice-llama-server-raspbian-arm64"}"

mkdir -p "$(dirname "$OUT_PATH")"
rm -rf "$OUT_PATH"

echo "Building Raspberry Pi OS (Linux ARM64) binary to: $OUT_PATH"
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -o "$OUT_PATH" ./cmd/nice-llama-server

file "$OUT_PATH"
