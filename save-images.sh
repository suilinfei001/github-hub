#!/bin/bash
# 在开发机上：将构建好的 Docker 镜像保存为 tar 文件

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  保存 Docker 镜像为 tar 文件${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

# 需要保存的镜像列表
IMAGES=(
    "ghh-server:latest"
    "quality-server:latest"
    "quality-frontend:latest"
    "mysql:latest"
    "nginx:latest"
    "alpine:3.19"
)

# 创建导出目录
EXPORT_DIR="${PROJECT_ROOT}/docker-images-export"
mkdir -p "$EXPORT_DIR"

echo -e "${YELLOW}保存镜像到: ${EXPORT_DIR}${NC}"
echo ""

for image in "${IMAGES[@]}"; do
    # 检查镜像是否存在
    if ! docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^${image}$"; then
        echo -e "  ${RED}✗${NC} 镜像不存在: ${image}"
        echo -e "  ${YELLOW}请先运行 ./prepare.sh 构建镜像${NC}"
        exit 1
    fi

    # 转换镜像名为文件名 (quality-server:latest -> quality-server_latest.tar)
    filename=$(echo "$image" | sed 's/:/_/g').tar
    filepath="${EXPORT_DIR}/${filename}"

    echo -e "  保存 ${YELLOW}${image}${NC}..."
    docker save -o "$filepath" "$image"

    size=$(du -h "$filepath" | cut -f1)
    echo -e "    ${GREEN}✓${NC} ${filename} (${size})"
done

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  保存完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}导出的镜像:${NC}"
ls -lh "$EXPORT_DIR" | grep -E "\.tar$" | awk '{print "  " $9 " (" $5 ")"}'
echo ""
echo -e "${YELLOW}下一步:${NC}"
echo -e "  1. 将整个项目目录（包括 docker-images-export/）拷贝到目标服务器"
echo -e "  2. 在目标服务器运行: ${GREEN}./start.sh${NC}"
echo ""
