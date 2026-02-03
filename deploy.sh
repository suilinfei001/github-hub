#!/bin/bash

# Quality Server 部署脚本
# 用于部署GitHub事件质量检查服务的Docker容器

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置变量
MYSQL_CONTAINER_NAME="mysql-ghh"
BACKEND_CONTAINER_NAME="ghh-server"
FRONTEND_CONTAINER_NAME="ghh-frontend"
MYSQL_ROOT_PASSWORD="rootpassword"
MYSQL_DATABASE="github_hub"
MYSQL_PORT=3306
BACKEND_PORT=5001
FRONTEND_PORT=80

# 函数：打印信息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

# 函数：打印警告
print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 函数：打印错误
print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 函数：检查并删除已存在的容器
check_and_remove_container() {
    local container_name=$1
    if docker ps -a --format '{{.Names}}' | grep -q "^${container_name}$"; then
        print_warning "容器 ${container_name} 已存在，正在删除..."
        docker rm -f ${container_name}
        print_info "容器 ${container_name} 已删除"
    fi
}

# 函数：检查本地镜像是否存在
check_local_image() {
    local image_name=$1
    if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^${image_name}$"; then
        print_info "本地镜像 ${image_name} 已存在"
        return 0
    else
        print_error "本地镜像 ${image_name} 不存在，请先下载该镜像"
        return 1
    fi
}

# 函数：等待MySQL就绪
wait_for_mysql() {
    print_info "等待MySQL服务启动..."
    local max_attempts=30
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if docker exec ${MYSQL_CONTAINER_NAME} mysqladmin ping -h localhost -u root -p${MYSQL_ROOT_PASSWORD} --silent; then
            print_info "MySQL服务已就绪"
            return 0
        fi
        echo -n "."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    print_error "MySQL服务启动超时"
    return 1
}

# 函数：检查端口是否被占用
check_port() {
    local port=$1
    local service_name=$2
    
    if netstat -tuln 2>/dev/null | grep -q ":${port} " || ss -tuln 2>/dev/null | grep -q ":${port} "; then
        print_warning "端口 ${port} 已被占用，${service_name} 可能无法正常启动"
    fi
}

# 主函数
main() {
    print_info "=========================================="
    print_info "  Quality Server 部署脚本"
    print_info "=========================================="
    echo ""
    
    # 检查Docker是否运行
    if ! docker info > /dev/null 2>&1; then
        print_error "Docker未运行，请先启动Docker"
        exit 1
    fi
    print_info "Docker运行正常"
    echo ""
    
    # 检查本地镜像
    print_info "检查本地Docker镜像..."
    check_local_image "nginx:latest" || exit 1
    check_local_image "golang:1.24-alpine" || exit 1
    check_local_image "alpine:3.19" || exit 1
    check_local_image "mysql:latest" || exit 1
    echo ""
    
    # 检查端口占用
    print_info "检查端口占用情况..."
    check_port ${MYSQL_PORT} "MySQL"
    check_port ${BACKEND_PORT} "Backend"
    check_port ${FRONTEND_PORT} "Frontend"
    echo ""
    
    # 步骤1：删除已存在的容器
    print_info "步骤1: 检查并删除已存在的容器..."
    check_and_remove_container ${MYSQL_CONTAINER_NAME}
    check_and_remove_container ${BACKEND_CONTAINER_NAME}
    check_and_remove_container ${FRONTEND_CONTAINER_NAME}
    echo ""
    
    # 步骤2：构建后端镜像
    print_info "步骤2: 构建后端Docker镜像..."
    if docker build -f Dockerfile.final -t ${BACKEND_CONTAINER_NAME} .; then
        print_info "后端镜像构建成功"
    else
        print_error "后端镜像构建失败"
        exit 1
    fi
    echo ""
    
    # 步骤3：启动MySQL容器
    print_info "步骤3: 启动MySQL容器..."
    if docker run -d \
        --name ${MYSQL_CONTAINER_NAME} \
        -p ${MYSQL_PORT}:3306 \
        -e MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD} \
        -e MYSQL_DATABASE=${MYSQL_DATABASE} \
        -v $(pwd)/mysql-data:/var/lib/mysql \
        mysql:latest; then
        print_info "MySQL容器启动成功"
    else
        print_error "MySQL容器启动失败"
        exit 1
    fi
    echo ""
    
    # 等待MySQL就绪
    wait_for_mysql
    echo ""
    
    # 步骤4：初始化数据库
    print_info "步骤4: 初始化数据库..."
    if [ -f "scripts/init-mysql.sql" ]; then
        if docker exec ${MYSQL_CONTAINER_NAME} mysql -uroot -p${MYSQL_ROOT_PASSWORD} < scripts/init-mysql.sql; then
            print_info "数据库初始化成功"
        else
            print_error "数据库初始化失败"
            exit 1
        fi
    else
        print_warning "数据库初始化脚本不存在，跳过初始化"
    fi
    echo ""
    
    # 步骤5：启动后端容器
    print_info "步骤5: 启动后端容器..."
    if docker run -d \
        --name ${BACKEND_CONTAINER_NAME} \
        -p ${BACKEND_PORT}:${BACKEND_PORT} \
        --link ${MYSQL_CONTAINER_NAME}:mysql \
        ${BACKEND_CONTAINER_NAME} \
        /app/quality-server -addr :${BACKEND_PORT} -db "root:${MYSQL_ROOT_PASSWORD}@tcp(mysql:3306)/${MYSQL_DATABASE}?parseTime=true"; then
        print_info "后端容器启动成功"
    else
        print_error "后端容器启动失败"
        exit 1
    fi
    echo ""
    
    # 等待后端服务就绪
    print_info "等待后端服务启动..."
    sleep 3
    echo ""
    
    # 步骤6：启动前端容器
    print_info "步骤6: 启动前端容器..."
    if docker run -d \
        --name ${FRONTEND_CONTAINER_NAME} \
        -p ${FRONTEND_PORT}:${FRONTEND_PORT} \
        ${FRONTEND_CONTAINER_NAME}; then
        print_info "前端容器启动成功"
    else
        print_error "前端容器启动失败"
        exit 1
    fi
    echo ""
    
    # 等待前端服务就绪
    print_info "等待前端服务启动..."
    sleep 2
    echo ""
    
    # 部署完成
    print_info "=========================================="
    print_info "  部署完成！"
    print_info "=========================================="
    echo ""
    print_info "服务访问地址："
    echo "  - 前端界面: http://localhost:${FRONTEND_PORT}"
    echo "  - 后端API: http://localhost:${BACKEND_PORT}"
    echo "  - Webhook: http://localhost:${BACKEND_PORT}/webhook"
    echo ""
    print_info "容器状态："
    docker ps --filter "name=${MYSQL_CONTAINER_NAME}" --filter "name=${BACKEND_CONTAINER_NAME}" --filter "name=${FRONTEND_CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    echo ""
    print_info "查看日志命令："
    echo "  - MySQL: docker logs ${MYSQL_CONTAINER_NAME}"
    echo "  - Backend: docker logs ${BACKEND_CONTAINER_NAME}"
    echo "  - Frontend: docker logs ${FRONTEND_CONTAINER_NAME}"
    echo ""
    print_info "停止服务命令："
    echo "  docker stop ${MYSQL_CONTAINER_NAME} ${BACKEND_CONTAINER_NAME} ${FRONTEND_CONTAINER_NAME}"
    echo ""
    print_info "删除容器命令："
    echo "  docker rm ${MYSQL_CONTAINER_NAME} ${BACKEND_CONTAINER_NAME} ${FRONTEND_CONTAINER_NAME}"
    echo ""
}

# 执行主函数
main