#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$ROOT/bin}"
GO_BIN="${GO:-go}"

mkdir -p "$BIN_DIR"

echo "Installing ghh-server -> $BIN_DIR/ghh-server"
$GO_BIN build -o "$BIN_DIR/ghh-server" "$ROOT/cmd/ghh-server"

echo "Installing ghh -> $BIN_DIR/ghh"
$GO_BIN build -o "$BIN_DIR/ghh" "$ROOT/cmd/ghh"

echo "Done."
