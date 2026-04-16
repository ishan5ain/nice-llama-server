#!/usr/bin/env bash

set -euo pipefail

# Build a macOS binary for the host architecture.
# Usage: build-macos.sh [output_path] [arch]
#   output_path: optional, defaults to dist/nice-llama-server-macos-<arch>
#   arch: optional, defaults to host architecture (amd64 or arm64)

# Show help if requested
if [[ "$1" == "-h" || "$1" == "--help" ]]; then
  cat <<EOF
Usage: $(basename "$0") [output_path] [arch]
  output_path: optional, defaults to dist/nice-llama-server-macos-<arch>
  arch: optional, defaults to host architecture (amd64 or arm64)
EOF
  exit 0
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Determine architecture: use provided second argument or auto-detect
if [[ -z "${2:-}" ]]; then
  case "$(uname -m)" in
    x86_64) ARCH=amd64 ;;
    arm64) ARCH=arm64 ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
else
  ARCH="$2"
fi

OUT_PATH="${1:-"$ROOT_DIR/dist/nice-llama-server-macos-${ARCH}"}"

mkdir -p "$(dirname "$OUT_PATH")"
rm -rf "$OUT_PATH"

echo "Building macOS binary to: $OUT_PATH"
GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=0 \
  go build -o "$OUT_PATH" ./cmd/nice-llama-server

file "$OUT_PATH"
