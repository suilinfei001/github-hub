#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

detect_os() {
  case "$(uname -s)" in
    Linux*) echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *) echo "" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "" ;;
  esac
}

pick_bin() {
  local name="$1"    # ghh
  local os="$2"
  local arch="$3"
  local root="$4"
  local candidate=""
  if [[ -n "$os" && -n "$arch" ]]; then
    candidate="$root/bin/${os}-${arch}/${name}"
    if [[ -x "$candidate" ]]; then
      echo "$candidate"
      return
    fi
  fi
  echo "$root/bin/${name}"
}

OS_NAME="$(detect_os)"
ARCH_NAME="$(detect_arch)"
CLIENT_BIN="$(pick_bin ghh "$OS_NAME" "$ARCH_NAME" "$ROOT")"

if [[ ! -x "$CLIENT_BIN" ]]; then
  echo "ghh binary not found under $ROOT/bin (looked for $CLIENT_BIN)" >&2
  exit 1
fi

exec "$CLIENT_BIN" "$@"

