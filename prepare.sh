#!/bin/bash
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

# 镜像仓库地址
REGISTRY="docker-images"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  GitHub-Hub 项目准备脚本${NC}"
echo -e "${GREEN}========================================${NC}"

# ============================================================
# 步骤 1: 加载基础镜像
# ============================================================
echo -e "\n${YELLOW}[步骤 1/5] 加载基础镜像...${NC}"

# 检查 docker-images 目录
IMAGES_DIR="${PROJECT_ROOT}/docker-images"
if [ ! -d "${IMAGES_DIR}" ]; then
    echo -e "  ${RED}✗${NC} docker-images 目录不存在: ${IMAGES_DIR}"
    exit 1
fi

load_image() {
    local tar_file=$1
    local expected_image=$2

    if [ ! -f "${IMAGES_DIR}/${tar_file}" ]; then
        echo -e "  ${YELLOW}!${NC} 镜像文件不存在: ${tar_file}，跳过"
        return 1
    fi

    echo -e "  加载 ${tar_file}..."
    if docker load -i "${IMAGES_DIR}/${tar_file}" > /tmp/docker_load_output.txt 2>&1; then
        # 解析加载后的镜像名称并打标签
        local loaded_image=$(grep -oP 'Loaded image: \K\S+' /tmp/docker_load_output.txt | head -1)
        if [ -n "$loaded_image" ]; then
            echo -e "  ${GREEN}✓${NC} ${tar_file} 加载成功 (${loaded_image})"
        else
            echo -e "  ${GREEN}✓${NC} ${tar_file} 加载成功"
        fi
        rm -f /tmp/docker_load_output.txt
        return 0
    else
        echo -e "  ${RED}✗${NC} ${tar_file} 加载失败"
        cat /tmp/docker_load_output.txt 2>/dev/null || true
        rm -f /tmp/docker_load_output.txt
        return 1
    fi
}

# 加载镜像（按依赖顺序）
load_image "alpine_3.19.tar" "alpine:3.19"
load_image "golang_1.24-alpine.tar" "golang:1.24-alpine"
load_image "nginx_latest.tar" "nginx:latest"
load_image "mysql_latest.tar" "mysql:latest"

echo -e "${GREEN}✓ 基础镜像加载完成${NC}"

# ============================================================
# 步骤 2: 编译 Go 二进制文件
# ============================================================
echo -e "\n${YELLOW}[步骤 2/5] 编译 Go 二进制文件...${NC}"

# 创建 bin 目录
mkdir -p bin

echo -e "  编译 ghh-server..."
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/ghh-server ./cmd/ghh-server

echo -e "  编译 ghh..."
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/ghh ./cmd/ghh

echo -e "  编译 quality-server..."
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/quality-server ./cmd/quality-server

echo -e "${GREEN}✓ Go 二进制文件编译完成${NC}"

# ============================================================
# 步骤 3: 构建前端
# ============================================================
echo -e "\n${YELLOW}[步骤 3/5] 构建前端...${NC}"

if [ ! -d "frontend" ]; then
    echo -e "  ${YELLOW}!${NC} frontend 目录不存在，跳过前端构建"
else
    echo -e "  配置 npm 国内镜像源..."
    cd frontend
    npm config set registry https://registry.npmmirror.com

    echo -e "  安装依赖..."
    if command -v pnpm &> /dev/null; then
        pnpm install
    elif command -v cnpm &> /dev/null; then
        cnpm install
    else
        npm install
    fi

    echo -e "  构建前端..."
    if command -v pnpm &> /dev/null; then
        pnpm build
    elif command -v cnpm &> /dev/null; then
        cnpm run build
    else
        npm run build
    fi
    cd "$PROJECT_ROOT"
    echo -e "${GREEN}✓ 前端构建完成${NC}"
fi

# ============================================================
# 步骤 4: 构建 Docker 镜像
# ============================================================
echo -e "\n${YELLOW}[步骤 4/5] 构建 Docker 镜像...${NC}"

# 构建 ghh-server 镜像
echo -e "  构建 ghh-server 镜像..."
docker build -f Dockerfile -t ghh-server:latest .
echo -e "  ${GREEN}✓${NC} ghh-server 镜像构建完成"

# 构建 quality-server 镜像
echo -e "  构建 quality-server 镜像..."
docker build -f Dockerfile_quality_server -t quality-server:latest .
echo -e "  ${GREEN}✓${NC} quality-server 镜像构建完成"

# 构建 quality-frontend 镜像
if [ -d "frontend/dist" ]; then
    echo -e "  构建 quality-frontend 镜像..."
    docker build -f Dockerfile_frontend -t quality-frontend:latest .
    echo -e "  ${GREEN}✓${NC} quality-frontend 镜像构建完成"
else
    echo -e "  ${YELLOW}!${NC} frontend/dist 不存在，跳过 quality-frontend 镜像构建"
fi

echo -e "${GREEN}✓ Docker 镜像构建完成${NC}"

# ============================================================
# 步骤 5: 创建必要的目录和数据卷
# ============================================================
echo -e "\n${YELLOW}[步骤 5/5] 创建必要的目录和数据卷...${NC}"

# 创建数据存储目录
mkdir -p data/ghh-server
mkdir -p data/quality-server
mkdir -p data/mysql

# 创建 Docker 网络（如果不存在）
if ! docker network inspect github-hub-network &> /dev/null; then
    docker network create github-hub-network
    echo -e "  ${GREEN}✓${NC} 创建 Docker 网络: github-hub-network"
else
    echo -e "  ${GREEN}✓${NC} Docker 网络: github-hub-network 已存在"
fi

echo -e "${GREEN}✓ 目录和数据卷准备完成${NC}"

# ============================================================
# 完成
# ============================================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}  准备工作完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "\n可用的镜像："
docker images | grep -E "ghh-server|quality-server|quality-frontend"
echo -e "\n下一步: 运行 ${YELLOW}./deploy.sh${NC} 启动所有容器\n"
