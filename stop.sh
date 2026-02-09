#!/bin/bash
# 在目标服务器上：停止所有容器

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}停止 GitHub-Hub 服务...${NC}"
echo ""

CONTAINERS=(
    "quality-frontend"
    "quality-server"
    "ghh-server"
    "quality-mysql"
)

for container in "${CONTAINERS[@]}"; do
    if docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
        echo -e "  停止 ${YELLOW}${container}${NC}..."
        docker stop "$container"
        docker rm "$container"
        echo -e "    ${GREEN}✓${NC} 已停止并删除"
    else
        echo -e "  ${container} 未运行"
    fi
done

echo ""
echo -e "${GREEN}✓ 所有服务已停止${NC}"
echo ""
