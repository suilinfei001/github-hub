# Quality Server 部署脚本 (PowerShell版本)
# 用于部署GitHub事件质量检查服务的Docker容器

# 配置变量
$MYSQL_CONTAINER_NAME = "mysql-ghh"
$BACKEND_CONTAINER_NAME = "ghh-server"
$FRONTEND_CONTAINER_NAME = "ghh-frontend"
$MYSQL_ROOT_PASSWORD = "rootpassword"
$MYSQL_DATABASE = "github_hub"
$MYSQL_PORT = 3306
$BACKEND_PORT = 5001
$FRONTEND_PORT = 80

# 函数：打印信息
function Print-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Green
}

# 函数：打印警告
function Print-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

# 函数：打印错误
function Print-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# 函数：检查并删除已存在的容器
function Check-AndRemove-Container {
    param([string]$ContainerName)
    $container = docker ps -a --format "{{.Names}}" | Select-String -Pattern "^$ContainerName$"
    if ($container) {
        Print-Warning "容器 $ContainerName 已存在，正在删除..."
        docker rm -f $ContainerName | Out-Null
        Print-Info "容器 $ContainerName 已删除"
    }
}

# 函数：检查本地镜像是否存在
function Check-Local-Image {
    param([string]$ImageName)
    $image = docker images --format "{{.Repository}}:{{.Tag}}" | Select-String -Pattern "^$ImageName$"
    if ($image) {
        Print-Info "本地镜像 $ImageName 已存在"
        return $true
    } else {
        Print-Error "本地镜像 $ImageName 不存在，请先下载该镜像"
        return $false
    }
}

# 函数：等待MySQL就绪
function Wait-For-MySQL {
    Print-Info "等待MySQL服务启动..."
    $maxAttempts = 30
    $attempt = 1
    
    while ($attempt -le $maxAttempts) {
        $result = docker exec $MYSQL_CONTAINER_NAME mysqladmin ping -h localhost -u root -p$MYSQL_ROOT_PASSWORD --silent 2>&1
        if ($LASTEXITCODE -eq 0) {
            Print-Info "MySQL服务已就绪"
            return $true
        }
        Write-Host "." -NoNewline
        Start-Sleep -Seconds 2
        $attempt++
    }
    
    Print-Error "MySQL服务启动超时"
    return $false
}

# 函数：检查端口是否被占用
function Check-Port {
    param([int]$Port, [string]$ServiceName)
    $connection = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
    if ($connection) {
        Print-Warning "端口 $Port 已被占用，$ServiceName 可能无法正常启动"
    }
}

# 主函数
function Main {
    Print-Info "=========================================="
    Print-Info "  Quality Server 部署脚本"
    Print-Info "=========================================="
    Write-Host ""
    
    # 检查Docker是否运行
    try {
        docker info | Out-Null
        Print-Info "Docker运行正常"
    } catch {
        Print-Error "Docker未运行，请先启动Docker"
        exit 1
    }
    Write-Host ""
    
    # 检查本地镜像
    Print-Info "检查本地Docker镜像..."
    if (-not (Check-Local-Image "nginx:latest")) { exit 1 }
    if (-not (Check-Local-Image "golang:1.24-alpine")) { exit 1 }
    if (-not (Check-Local-Image "alpine:3.19")) { exit 1 }
    if (-not (Check-Local-Image "mysql:latest")) { exit 1 }
    Write-Host ""
    
    # 检查端口占用
    Print-Info "检查端口占用情况..."
    Check-Port $MYSQL_PORT "MySQL"
    Check-Port $BACKEND_PORT "Backend"
    Check-Port $FRONTEND_PORT "Frontend"
    Write-Host ""
    
    # 步骤1：删除已存在的容器
    Print-Info "步骤1: 检查并删除已存在的容器..."
    Check-AndRemove-Container $MYSQL_CONTAINER_NAME
    Check-AndRemove-Container $BACKEND_CONTAINER_NAME
    Check-AndRemove-Container $FRONTEND_CONTAINER_NAME
    Write-Host ""
    
    # 步骤2：构建后端镜像
    Print-Info "步骤2: 构建后端Docker镜像..."
    $buildResult = docker build -f Dockerfile.final -t $BACKEND_CONTAINER_NAME . 2>&1
    if ($LASTEXITCODE -eq 0) {
        Print-Info "后端镜像构建成功"
    } else {
        Print-Error "后端镜像构建失败"
        Write-Host $buildResult
        exit 1
    }
    Write-Host ""
    
    # 步骤3：启动MySQL容器
    Print-Info "步骤3: 启动MySQL容器..."
    $currentDir = Get-Location
    $mysqlResult = docker run -d `
        --name $MYSQL_CONTAINER_NAME `
        -p ${MYSQL_PORT}:3306 `
        -e MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD `
        -e MYSQL_DATABASE=$MYSQL_DATABASE `
        -v "${currentDir}\mysql-data:/var/lib/mysql" `
        mysql:latest 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Print-Info "MySQL容器启动成功"
    } else {
        Print-Error "MySQL容器启动失败"
        Write-Host $mysqlResult
        exit 1
    }
    Write-Host ""
    
    # 等待MySQL就绪
    Wait-For-MySQL
    Write-Host ""
    
    # 步骤4：初始化数据库
    Print-Info "步骤4: 初始化数据库..."
    if (Test-Path "scripts\init-mysql.sql") {
        $initResult = docker exec $MYSQL_CONTAINER_NAME mysql -uroot -p$MYSQL_ROOT_PASSWORD < scripts\init-mysql.sql 2>&1
        if ($LASTEXITCODE -eq 0) {
            Print-Info "数据库初始化成功"
        } else {
            Print-Error "数据库初始化失败"
            Write-Host $initResult
            exit 1
        }
    } else {
        Print-Warning "数据库初始化脚本不存在，跳过初始化"
    }
    Write-Host ""
    
    # 步骤5：启动后端容器
    Print-Info "步骤5: 启动后端容器..."
    $backendResult = docker run -d `
        --name $BACKEND_CONTAINER_NAME `
        -p ${BACKEND_PORT}:${BACKEND_PORT} `
        --link ${MYSQL_CONTAINER_NAME}:mysql `
        $BACKEND_CONTAINER_NAME `
        /app/quality-server -addr :${BACKEND_PORT} -db "root:${MYSQL_ROOT_PASSWORD}@tcp(mysql:3306)/${MYSQL_DATABASE}?parseTime=true" 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Print-Info "后端容器启动成功"
    } else {
        Print-Error "后端容器启动失败"
        Write-Host $backendResult
        exit 1
    }
    Write-Host ""
    
    # 等待后端服务就绪
    Print-Info "等待后端服务启动..."
    Start-Sleep -Seconds 3
    Write-Host ""
    
    # 步骤6：启动前端容器
    Print-Info "步骤6: 启动前端容器..."
    $frontendResult = docker run -d `
        --name $FRONTEND_CONTAINER_NAME `
        -p ${FRONTEND_PORT}:${FRONTEND_PORT} `
        $FRONTEND_CONTAINER_NAME 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Print-Info "前端容器启动成功"
    } else {
        Print-Error "前端容器启动失败"
        Write-Host $frontendResult
        exit 1
    }
    Write-Host ""
    
    # 等待前端服务就绪
    Print-Info "等待前端服务启动..."
    Start-Sleep -Seconds 2
    Write-Host ""
    
    # 部署完成
    Print-Info "=========================================="
    Print-Info "  部署完成！"
    Print-Info "=========================================="
    Write-Host ""
    Print-Info "服务访问地址："
    Write-Host "  - 前端界面: http://localhost:${FRONTEND_PORT}"
    Write-Host "  - 后端API: http://localhost:${BACKEND_PORT}"
    Write-Host "  - Webhook: http://localhost:${BACKEND_PORT}/webhook"
    Write-Host ""
    Print-Info "容器状态："
    docker ps --filter "name=$MYSQL_CONTAINER_NAME" --filter "name=$BACKEND_CONTAINER_NAME" --filter "name=$FRONTEND_CONTAINER_NAME" --format "table {{.Names}}`t{{.Status}}`t{{.Ports}}"
    Write-Host ""
    Print-Info "查看日志命令："
    Write-Host "  - MySQL: docker logs $MYSQL_CONTAINER_NAME"
    Write-Host "  - Backend: docker logs $BACKEND_CONTAINER_NAME"
    Write-Host "  - Frontend: docker logs $FRONTEND_CONTAINER_NAME"
    Write-Host ""
    Print-Info "停止服务命令："
    Write-Host "  docker stop $MYSQL_CONTAINER_NAME $BACKEND_CONTAINER_NAME $FRONTEND_CONTAINER_NAME"
    Write-Host ""
    Print-Info "删除容器命令："
    Write-Host "  docker rm $MYSQL_CONTAINER_NAME $BACKEND_CONTAINER_NAME $FRONTEND_CONTAINER_NAME"
    Write-Host ""
}

# 执行主函数
Main