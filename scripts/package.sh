#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT/bin"
INSTALL_SH="$ROOT/install.sh"
CONFIG_CLIENT="$ROOT/configs/config.example.yaml"
CONFIG_SERVER="$ROOT/configs/server.config.example.yaml"
DIST_DIR="${DIST_DIR:-$ROOT/dist}"
VERSION_INPUT="${1:-}"
VERSION="${VERSION_INPUT:-${VERSION:-main}}"
ARCHIVE_NAME="${ARCHIVE_NAME:-ghh-$VERSION.tar.gz}"
ARCHIVE_PATH="$DIST_DIR/$ARCHIVE_NAME"

if [[ ! -d "$BIN_DIR" ]] || [[ -z "$(ls -A "$BIN_DIR" 2>/dev/null)" ]]; then
  echo "bin/ 为空，请先构建客户端和服务端（二进制）" >&2
  exit 1
fi

if [[ ! -f "$INSTALL_SH" ]]; then
  echo "install.sh 不存在，无法打包" >&2
  exit 1
fi

missing_cfg=0
if [[ ! -f "$CONFIG_CLIENT" ]]; then
  echo "缺少客户端配置模板：configs/config.example.yaml" >&2
  missing_cfg=1
fi
if [[ ! -f "$CONFIG_SERVER" ]]; then
  echo "缺少服务端配置模板：configs/server.config.example.yaml" >&2
  missing_cfg=1
fi
if [[ $missing_cfg -ne 0 ]]; then
  exit 1
fi

mkdir -p "$DIST_DIR"

# 打包 bin 内所有文件、install.sh 以及配置模板
tar -czf "$ARCHIVE_PATH" \
  -C "$ROOT" install.sh \
  -C "$ROOT" bin \
  -C "$ROOT" configs/config.example.yaml configs/server.config.example.yaml \
  -C "$ROOT" README.app.md README.app.zh.md

echo "已生成归档：$ARCHIVE_PATH"

