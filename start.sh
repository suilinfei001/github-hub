#!/bin/bash
# 在目标服务器上：加载镜像并启动所有容器
#
# 使用方法:
#   ./start.sh           # 升级模式（默认）：更新现有容器
#   ./start.sh -u        # 升级模式：更新现有容器
#   ./start.sh -r        # 恢复模式：完全卸载后重装，清理数据库

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ============================================================
# 参数解析
# ============================================================
MODE="upgrade"
FORCE_CLEANUP=false

while getopts "rfh" opt; do
    case $opt in
        r)
            MODE="recover"
            FORCE_CLEANUP=true
            ;;
        h)
            echo "使用方法:"
            echo "  ./start.sh           # 升级模式（默认）：更新现有容器"
            echo "  ./start.sh -u        # 升级模式：更新现有容器"
            echo "  ./start.sh -r        # 恢复模式：完全卸载后重装，清理数据库"
            echo "  ./start.sh -h        # 显示帮助信息"
            exit 0
            ;;
        \?)
            echo -e "${RED}无效选项: -$OPTARG${NC}"
            echo "使用 -h 查看帮助信息"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  GitHub-Hub 服务部署脚本${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

if [ "$MODE" = "recover" ]; then
    echo -e "${YELLOW}模式: ${RED}恢复模式 (完全重装，清理数据库)${NC}"
    echo -e "${RED}警告: 此操作将删除所有容器和数据库数据！${NC}"
    echo -ne "确认继续? [y/N] "
    read -r confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        echo "操作已取消"
        exit 0
    fi
else
    echo -e "${YELLOW}模式: ${GREEN}升级模式 (更新容器，保留数据)${NC}"
fi
echo ""

PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

# ============================================================
# 配置
# ============================================================
NETWORK_NAME="github-hub-network"
MYSQL_ROOT_PASSWORD="root123456"
MYSQL_DATABASE="github_hub"
MYSQL_PORT=3307
QUALITY_SERVER_PORT=5001
GHH_SERVER_PORT=8080
FRONTEND_PORT=8081

CONTAINERS=(
    "quality-frontend"
    "quality-server"
    "ghh-server"
    "quality-mysql"
)

PORTS=("$MYSQL_PORT" "$QUALITY_SERVER_PORT" "$GHH_SERVER_PORT" "$FRONTEND_PORT")

# ============================================================
# 工具函数
# ============================================================

# 检查端口是否被占用
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 || \
       netstat -tuln 2>/dev/null | grep -q ":$port " || \
       ss -tuln 2>/dev/null | grep -q ":$port "; then
        return 0
    fi
    return 1
}

# 获取占用端口的进程信息
get_port_process() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        lsof -Pi :$port -sTCP:LISTEN 2>/dev/null | tail -n +2
    elif command -v netstat >/dev/null 2>&1; then
        netstat -tuln 2>/dev/null | grep ":$port "
    elif command -v ss >/dev/null 2>&1; then
        ss -tuln 2>/dev/null | grep ":$port "
    fi
}

# 检查容器是否存在
container_exists() {
    local name=$1
    docker ps -a --format '{{.Names}}' | grep -q "^${name}$"
}

# 检查容器是否运行中
container_running() {
    local name=$1
    docker ps --format '{{.Names}}' | grep -q "^${name}$"
}

# 获取容器状态
get_container_status() {
    local name=$1
    if container_running "$name"; then
        echo "running"
    elif container_exists "$name"; then
        echo "exited"
    else
        echo "nonexistent"
    fi
}

# ============================================================
# 预检查：端口占用检测
# ============================================================
echo -e "${YELLOW}[预检查] 检测端口占用...${NC}"

PORTS_OK=true
for port in "${PORTS[@]}"; do
    if check_port "$port"; then
        # 检查是否是我们的容器占用
        PORT_OWNED=false

        # 检查 MySQL 端口
        if [ "$port" = "$MYSQL_PORT" ] && container_running "quality-mysql"; then
            PORT_OWNED=true
        fi

        # 检查其他端口
        for container in "${CONTAINERS[@]}"; do
            if docker port "$container" 2>/dev/null | grep -q ":$port->"; then
                PORT_OWNED=true
                break
            fi
        done

        if [ "$PORT_OWNED" = false ]; then
            echo -e "  ${RED}✗${NC} 端口 ${YELLOW}$port${NC} 被占用："
            get_port_process "$port" | sed 's/^/    /'
            PORTS_OK=false
        fi
    fi
done

if [ "$PORTS_OK" = false ]; then
    echo ""
    echo -e "${RED}错误: 检测到端口冲突！${NC}"
    echo -e "请先停止占用端口的进程，或使用 ${YELLOW}./stop.sh${NC} 停止本项目的容器"
    exit 1
fi

echo -e "  ${GREEN}✓${NC} 所有端口检测通过"
echo ""

# ============================================================
# 预检查：容器状态检测
# ============================================================
echo -e "${YELLOW}[预检查] 检测容器状态...${NC}"

for container in "${CONTAINERS[@]}"; do
    status=$(get_container_status "$container")
    case $status in
        running)
            echo -e "  ${GREEN}●${NC} $container: 运行中"
            ;;
        exited)
            echo -e "  ${YELLOW}○${NC} $container: 已停止"
            ;;
        nonexistent)
            echo -e "  ${BLUE}+${NC} $container: 不存在"
            ;;
    esac
done
echo ""

# ============================================================
# 恢复模式：完全清理
# ============================================================
if [ "$MODE" = "recover" ]; then
    echo -e "${YELLOW}[恢复模式] 清理现有容器和数据...${NC}"
    echo ""

    # 停止并删除所有容器
    for container in "${CONTAINERS[@]}"; do
        if container_exists "$container"; then
            echo -e "  删除容器 ${YELLOW}${container}${NC}..."
            docker stop "$container" >/dev/null 2>&1 || true
            docker rm "$container" >/dev/null 2>&1
            echo -e "    ${GREEN}✓${NC} 已删除"
        fi
    done

    # 清理数据库数据（警告：不可恢复！）
    echo -e "  清理数据库数据..."
    if [ -d "${PROJECT_ROOT}/data/mysql" ]; then
        # 备份数据目录
        BACKUP_DIR="${PROJECT_ROOT}/data/mysql.backup.$(date +%Y%m%d_%H%M%S)"
        mv "${PROJECT_ROOT}/data/mysql" "$BACKUP_DIR"
        echo -e "    ${GREEN}✓${NC} 数据已备份到: $BACKUP_DIR"
    fi

    # 重建空的数据目录
    mkdir -p "${PROJECT_ROOT}/data/mysql"
    echo -e "    ${GREEN}✓${NC} 数据库目录已重置"
    echo ""

    # 删除并重建网络
    if docker network inspect "$NETWORK_NAME" &>/dev/null; then
        docker network rm "$NETWORK_NAME" >/dev/null 2>&1
        echo -e "  ${GREEN}✓${NC} Docker 网络已删除"
    fi
fi

# ============================================================
# 步骤 1: 加载 Docker 镜像
# ============================================================
echo -e "${YELLOW}[步骤 1/5] 加载 Docker 镜像...${NC}"

EXPORT_DIR="${PROJECT_ROOT}/docker-images-export"
if [ ! -d "$EXPORT_DIR" ]; then
    echo -e "  ${RED}✗${NC} 镜像目录不存在: $EXPORT_DIR"
    echo -e "  ${YELLOW}请先在开发机运行 ./save-images.sh${NC}"
    exit 1
fi

loaded_count=0
for tar_file in "$EXPORT_DIR"/*.tar; do
    if [ -f "$tar_file" ]; then
        filename=$(basename "$tar_file")
        echo -e "  加载 ${YELLOW}${filename}${NC}..."
        docker load -i "$tar_file" > /dev/null 2>&1
        echo -e "    ${GREEN}✓${NC} 完成"
        loaded_count=$((loaded_count + 1))
    fi
done

echo -e "  ${GREEN}✓${NC} 已加载 $loaded_count 个镜像"
echo ""

# ============================================================
# 步骤 2: 创建 Docker 网络
# ============================================================
echo -e "${YELLOW}[步骤 2/5] 创建 Docker 网络...${NC}"

if ! docker network inspect "$NETWORK_NAME" &> /dev/null; then
    docker network create "$NETWORK_NAME"
    echo -e "  ${GREEN}✓${NC} 创建网络: $NETWORK_NAME"
else
    echo -e "  ${GREEN}✓${NC} 网络 $NETWORK_NAME 已存在"
fi
echo ""

# ============================================================
# 步骤 3: 创建数据目录
# ============================================================
echo -e "${YELLOW}[步骤 3/5] 创建数据目录...${NC}"

mkdir -p "${PROJECT_ROOT}/data/mysql"
mkdir -p "${PROJECT_ROOT}/data/quality-server"
mkdir -p "${PROJECT_ROOT}/data/ghh-server"
echo -e "  ${GREEN}✓${NC} 数据目录准备完成"
echo ""

# ============================================================
# 步骤 4: 启动/升级容器
# ============================================================
echo -e "${YELLOW}[步骤 4/5] 启动容器...${NC}"

# 等待函数
wait_for() {
    local name=$1
    local max_wait=${2:-30}
    local count=0

    echo -ne "    等待 ${name} 启动"
    while [ $count -lt $max_wait ]; do
        if docker ps --format '{{.Names}}' | grep -q "^${name}$"; then
            echo -e "\r    ${GREEN}✓${NC} ${name} 已启动"
            return 0
        fi
        sleep 1
        count=$((count + 1))
        echo -ne "."
    done
    echo -e "\r    ${RED}✗${NC} ${name} 启动超时"
    return 1
}

# 升级容器函数
upgrade_container() {
    local container_name=$1
    local new_image=$2
    shift 2
    local run_cmd="$@"

    local status=$(get_container_status "$container_name")
    local needs_start=false
    local needs_recreate=false

    case $status in
        running)
            # 检查是否需要升级（镜像是否更新）
            local current_image=$(docker inspect "$container_name" --format='{{.Config.Image}}')
            if [ "$current_image" != "$new_image" ]; then
                echo -e "  升级 ${YELLOW}${container_name}${NC} (镜像变更)..."
                needs_recreate=true
            else
                echo -e "  ${container_name}: ${GREEN}已是最新版本${NC}"
                return 0
            fi
            ;;
        exited)
            echo -e "  重启 ${YELLOW}${container_name}${NC}..."
            needs_start=true
            ;;
        nonexistent)
            echo -e "  创建 ${YELLOW}${container_name}${NC}..."
            needs_recreate=true
            ;;
    esac

    if [ "$needs_recreate" = true ]; then
        # 停止并删除旧容器
        if container_exists "$container_name"; then
            docker stop "$container_name" >/dev/null 2>&1 || true
            docker rm "$container_name" >/dev/null 2>&1 || true
        fi

        # 创建新容器
        eval $run_cmd
        wait_for "$container_name"
    elif [ "$needs_start" = true ]; then
        docker start "$container_name"
        wait_for "$container_name"
    fi
}

# MySQL 容器
upgrade_container "quality-mysql" "mysql:latest" "
docker run -d \
    --name quality-mysql \
    --network $NETWORK_NAME \
    -p ${MYSQL_PORT}:3306 \
    -e MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD \
    -e MYSQL_DATABASE=$MYSQL_DATABASE \
    -v \${PROJECT_ROOT}/data/mysql:/var/lib/mysql \
    -v \${PROJECT_ROOT}/scripts/init-mysql.sql:/docker-entrypoint-initdb.d/init.sql \
    --restart unless-stopped \
    mysql:latest \
    --character-set-server=utf8mb4 \
    --collation-server=utf8mb4_unicode_ci
"

# 等待 MySQL 就绪（仅在新创建时）
if [ "$(get_container_status quality-mysql)" = "running" ]; then
    echo -ne "    等待 MySQL 就绪"
    for i in {1..30}; do
        if docker exec quality-mysql mysqladmin ping -h localhost -uroot -p"$MYSQL_ROOT_PASSWORD" &> /dev/null; then
            echo -e "\r    ${GREEN}✓${NC} MySQL 已就绪"
            break
        fi
        sleep 1
        echo -ne "."
    done
fi

# Quality Server 容器
upgrade_container "quality-server" "quality-server:latest" "
docker run -d \
    --name quality-server \
    --network $NETWORK_NAME \
    -p ${QUALITY_SERVER_PORT}:5001 \
    -v \${PROJECT_ROOT}/data/quality-server:/data \
    --restart unless-stopped \
    quality-server:latest \
    /app/quality-server \
    -addr :5001 \
    -db \"root:${MYSQL_ROOT_PASSWORD}@tcp(quality-mysql:3306)/${MYSQL_DATABASE}?parseTime=true\" \
    -log-level info
"

# Frontend 容器
upgrade_container "quality-frontend" "quality-frontend:latest" "
docker run -d \
    --name quality-frontend \
    --network $NETWORK_NAME \
    -p ${FRONTEND_PORT}:80 \
    --restart unless-stopped \
    quality-frontend:latest
"

# GHH Server 容器
upgrade_container "ghh-server" "ghh-server:latest" "
docker run -d \
    --name ghh-server \
    --network $NETWORK_NAME \
    -p ${GHH_SERVER_PORT}:8080 \
    -e GITHUB_TOKEN=\"\" \
    -v \${PROJECT_ROOT}/data/ghh-server:/data \
    --restart unless-stopped \
    ghh-server:latest \
    --addr :8080 \
    --root /data
"

echo ""
echo -e "  ${GREEN}✓${NC} 所有容器启动完成"
echo ""

# ============================================================
# 步骤 5: 显示状态
# ============================================================
echo -e "${YELLOW}[步骤 5/5] 服务状态${NC}"
echo ""

docker ps --filter "network=${NETWORK_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  部署完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}访问地址:${NC}"
echo -e "  Frontend:       ${GREEN}http://$(hostname -I | awk '{print $1'}):${FRONTEND_PORT}${NC}"
echo -e "  Quality API:    ${GREEN}http://$(hostname -I | awk '{print $1'}):${QUALITY_SERVER_PORT}${NC}"
echo -e "  GHH Server:     ${GREEN}http://$(hostname -I | awk '{print $1'}):${GHH_SERVER_PORT}${NC}"
echo -e "  MySQL:          ${GREEN}localhost:${MYSQL_PORT}${NC}"
echo ""
echo -e "${BLUE}数据库连接信息:${NC}"
echo -e "  Host: quality-mysql"
echo -e "  Port: 3306"
echo -e "  Database: $MYSQL_DATABASE"
echo -e "  Username: root"
echo -e "  Password: $MYSQL_ROOT_PASSWORD"
echo ""
echo -e "${BLUE}常用命令:${NC}"
echo -e "  查看日志:       docker logs -f <container-name>"
echo -e "  停止服务:       ./stop.sh"
echo -e "  重启服务:       ./stop.sh && ./start.sh"
echo -e "  完全重装:       ./start.sh -r"
echo ""
