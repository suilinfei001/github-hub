#!/bin/bash
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

# 配置
NETWORK_NAME="github-hub-network"
MYSQL_ROOT_PASSWORD="root123456"
MYSQL_DATABASE="github_hub"
MYSQL_CONTAINER="quality-mysql"
QUALITY_SERVER_CONTAINER="quality-server"
QUALITY_FRONTEND_CONTAINER="quality-frontend"
GHH_SERVER_CONTAINER="ghh-server"

# 端口配置
MYSQL_PORT=3307
QUALITY_SERVER_PORT=5001
QUALITY_FRONTEND_PORT=8081
GHH_SERVER_PORT=8080

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  GitHub-Hub 容器部署脚本${NC}"
echo -e "${GREEN}========================================${NC}"

# ============================================================
# 函数定义
# ============================================================

# 检查容器是否运行
is_container_running() {
    local container=$1
    if docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
        return 0
    else
        return 1
    fi
}

# 检查容器是否存在（无论是否运行）
is_container_exists() {
    local container=$1
    if docker ps -a --format '{{.Names}}' | grep -q "^${container}$"; then
        return 0
    else
        return 1
    fi
}

# 等待容器就绪
wait_for_container() {
    local container=$1
    local max_wait=30
    local count=0

    echo -ne "  等待 ${container} 启动"
    while [ $count -lt $max_wait ]; do
        if is_container_running "$container"; then
            echo -e "\r  ${GREEN}✓${NC} ${container} 已启动"
            return 0
        fi
        sleep 1
        count=$((count + 1))
        echo -ne "."
    done
    echo -e "\r  ${RED}✗${NC} ${container} 启动超时"
    return 1
}

# 等待 MySQL 就绪
wait_for_mysql() {
    local max_wait=60
    local count=0

    echo -ne "  等待 MySQL 数据库就绪"
    while [ $count -lt $max_wait ]; do
        if docker exec ${MYSQL_CONTAINER} mysqladmin ping -h localhost -uroot -p${MYSQL_ROOT_PASSWORD} &> /dev/null; then
            echo -e "\r  ${GREEN}✓${NC} MySQL 数据库已就绪"
            return 0
        fi
        sleep 1
        count=$((count + 1))
        echo -ne "."
    done
    echo -e "\r  ${RED}✗${NC} MySQL 启动超时"
    return 1
}

# ============================================================
# 检查 Docker 网络
# ============================================================
echo -e "\n${YELLOW}[检查] Docker 网络...${NC}"
if ! docker network inspect ${NETWORK_NAME} &> /dev/null; then
    echo -e "  创建 Docker 网络: ${NETWORK_NAME}"
    docker network create ${NETWORK_NAME}
else
    echo -e "  ${GREEN}✓${NC} Docker 网络 ${NETWORK_NAME} 已存在"
fi

# ============================================================
# 容器 1: quality-mysql
# ============================================================
echo -e "\n${YELLOW}[容器 1/4] quality-mysql${NC}"

if is_container_running "${MYSQL_CONTAINER}"; then
    echo -e "  ${YELLOW}!${NC} ${MYSQL_CONTAINER} 已在运行，跳过"
else
    # 如果容器存在但未运行，先删除
    if is_container_exists "${MYSQL_CONTAINER}"; then
        echo -e "  删除已存在的 ${MYSQL_CONTAINER} 容器"
        docker rm -f ${MYSQL_CONTAINER} &> /dev/null || true
    fi

    echo -e "  启动 ${MYSQL_CONTAINER}..."
    docker run -d \
        --name ${MYSQL_CONTAINER} \
        --network ${NETWORK_NAME} \
        -p ${MYSQL_PORT}:3306 \
        -e MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD} \
        -e MYSQL_DATABASE=${MYSQL_DATABASE} \
        -v "${PROJECT_ROOT}/data/mysql:/var/lib/mysql" \
        -v "${PROJECT_ROOT}/scripts/init-mysql.sql:/docker-entrypoint-initdb.d/init.sql" \
        --restart unless-stopped \
        mysql:latest \
        --character-set-server=utf8mb4 \
        --collation-server=utf8mb4_unicode_ci

    wait_for_mysql || exit 1
fi

# ============================================================
# 容器 2: quality-server
# ============================================================
echo -e "\n${YELLOW}[容器 2/4] quality-server${NC}"

if is_container_running "${QUALITY_SERVER_CONTAINER}"; then
    echo -e "  ${YELLOW}!${NC} ${QUALITY_SERVER_CONTAINER} 已在运行，跳过"
else
    # 如果容器存在但未运行，先删除
    if is_container_exists "${QUALITY_SERVER_CONTAINER}"; then
        echo -e "  删除已存在的 ${QUALITY_SERVER_CONTAINER} 容器"
        docker rm -f ${QUALITY_SERVER_CONTAINER} &> /dev/null || true
    fi

    echo -e "  启动 ${QUALITY_SERVER_CONTAINER}..."
    docker run -d \
        --name ${QUALITY_SERVER_CONTAINER} \
        --network ${NETWORK_NAME} \
        -p ${QUALITY_SERVER_PORT}:5001 \
        -e GITHUB_TOKEN="" \
        -v "${PROJECT_ROOT}/data/quality-server:/data" \
        --restart unless-stopped \
        quality-server:latest \
        /app/quality-server \
        -addr :5001 \
        -db "root:${MYSQL_ROOT_PASSWORD}@tcp(${MYSQL_CONTAINER}:3306)/${MYSQL_DATABASE}?parseTime=true" \
        -log-level info
        # 可选日志参数:
        # -log-level (debug|info|warn|error) - 默认: info
        # -log-json - 使用 JSON 格式日志 - 默认: false
        # -log-no-color - 禁用彩色日志输出 - 默认: false

    wait_for_container "${QUALITY_SERVER_CONTAINER}" || exit 1
fi

# ============================================================
# 容器 3: quality-frontend
# ============================================================
echo -e "\n${YELLOW}[容器 3/4] quality-frontend${NC}"

if is_container_running "${QUALITY_FRONTEND_CONTAINER}"; then
    echo -e "  ${YELLOW}!${NC} ${QUALITY_FRONTEND_CONTAINER} 已在运行，跳过"
else
    # 检查镜像是否存在
    if ! docker images quality-frontend:latest | grep -q quality-frontend; then
        echo -e "  ${YELLOW}!${NC} quality-frontend:latest 镜像不存在，跳过"
        echo -e "  请先运行 ${YELLOW}./prepare.sh${NC} 构建前端镜像"
    else
        # 如果容器存在但未运行，先删除
        if is_container_exists "${QUALITY_FRONTEND_CONTAINER}"; then
            echo -e "  删除已存在的 ${QUALITY_FRONTEND_CONTAINER} 容器"
            docker rm -f ${QUALITY_FRONTEND_CONTAINER} &> /dev/null || true
        fi

        echo -e "  启动 ${QUALITY_FRONTEND_CONTAINER}..."
        docker run -d \
            --name ${QUALITY_FRONTEND_CONTAINER} \
            --network ${NETWORK_NAME} \
            -p ${QUALITY_FRONTEND_PORT}:80 \
            --restart unless-stopped \
            quality-frontend:latest

        wait_for_container "${QUALITY_FRONTEND_CONTAINER}" || exit 1
    fi
fi

# ============================================================
# 容器 4: ghh-server
# ============================================================
echo -e "\n${YELLOW}[容器 4/4] ghh-server${NC}"

if is_container_running "${GHH_SERVER_CONTAINER}"; then
    echo -e "  ${YELLOW}!${NC} ${GHH_SERVER_CONTAINER} 已在运行，跳过"
else
    # 如果容器存在但未运行，先删除
    if is_container_exists "${GHH_SERVER_CONTAINER}"; then
        echo -e "  删除已存在的 ${GHH_SERVER_CONTAINER} 容器"
        docker rm -f ${GHH_SERVER_CONTAINER} &> /dev/null || true
    fi

    echo -e "  启动 ${GHH_SERVER_CONTAINER}..."
    docker run -d \
        --name ${GHH_SERVER_CONTAINER} \
        --network ${NETWORK_NAME} \
        -p ${GHH_SERVER_PORT}:8080 \
        -e GITHUB_TOKEN="" \
        -v "${PROJECT_ROOT}/data/ghh-server:/data" \
        --restart unless-stopped \
        ghh-server:latest \
        --addr :8080 \
        --root /data

    wait_for_container "${GHH_SERVER_CONTAINER}" || exit 1
fi

# ============================================================
# 显示容器状态
# ============================================================
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}  部署完成！${NC}"
echo -e "${GREEN}========================================${NC}"

echo -e "\n${BLUE}容器状态:${NC}"
docker ps --filter "network=${NETWORK_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo -e "\n${BLUE}服务访问地址:${NC}"
echo -e "  ghh-server:      ${GREEN}http://localhost:${GHH_SERVER_PORT}${NC}"
echo -e "  quality-server:  ${GREEN}http://localhost:${QUALITY_SERVER_PORT}${NC}"
echo -e "  quality-frontend:${GREEN}http://localhost:${QUALITY_FRONTEND_PORT}${NC}"
echo -e "  MySQL:           ${GREEN}localhost:${MYSQL_PORT}${NC}"
echo -e "    Database: ${MYSQL_DATABASE}"
echo -e "    Username: root"
echo -e "    Password: ${MYSQL_ROOT_PASSWORD}"

echo -e "\n${BLUE}常用命令:${NC}"
echo -e "  查看日志: docker logs -f <container-name>"
echo -e "  停止所有: docker stop ${GHH_SERVER_CONTAINER} ${QUALITY_SERVER_CONTAINER} ${QUALITY_FRONTEND_CONTAINER} ${MYSQL_CONTAINER}"
echo -e "  删除所有: docker rm -f ${GHH_SERVER_CONTAINER} ${QUALITY_SERVER_CONTAINER} ${QUALITY_FRONTEND_CONTAINER} ${MYSQL_CONTAINER}"
echo ""
