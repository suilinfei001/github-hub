# Quality Server

GitHub事件质量检查服务，用于监控和处理GitHub Pull Request和Push事件，自动执行质量检查流程。

## 项目简介

Quality Server是一个基于Go语言开发的GitHub Webhook服务，用于接收和处理GitHub事件，自动执行代码质量检查流程。系统支持多种检查类型，包括编译检查、代码规范检查、安全扫描、单元测试、API测试、端到端测试等。

## 功能特性

- **GitHub Webhook集成**: 接收和处理GitHub Pull Request和Push事件
- **智能事件过滤**: 只处理符合条件的PR和Push事件
- **多阶段质量检查**: 支持基础CI、部署、专项测试三个阶段的检查
- **MySQL数据持久化**: 使用MySQL数据库存储事件和质量检查数据
- **RESTful API**: 提供完整的API接口用于查询和管理数据
- **Docker容器化**: 支持Docker容器部署，便于运维管理

## 系统架构

### 容器架构

项目包含3个Docker容器：

1. **ghh-server** (后端服务)
   - 端口: 5001
   - 功能: 处理GitHub Webhook事件，执行质量检查
   - 存储: MySQL数据库

2. **ghh-frontend** (前端服务)
   - 端口: 80
   - 功能: 提供Web界面展示质量检查结果
   - 镜像: nginx:latest

3. **mysql** (数据库服务)
   - 端口: 3306
   - 功能: 存储GitHub事件和质量检查数据
   - 镜像: mysql:latest
   - 数据持久化: 本地mysql-data目录

### 质量检查阶段

系统将质量检查分为三个阶段：

1. **基础CI (Basic CI)**
   - 编译检查 (compilation)
   - 代码规范检查 (code_lint)
   - 安全扫描 (security_scan)
   - 单元测试 (unit_test)

2. **部署 (Deployment)**
   - 部署检查 (deployment)

3. **专项测试 (Specialized Tests)**
   - API测试 (api_test)
   - 模块端到端测试 (module_e2e)
   - 代理端到端测试 (agent_e2e)
   - AI端到端测试 (ai_e2e)

## 技术栈

- **后端**: Go 1.24
- **数据库**: MySQL 9.6
- **前端**: Nginx
- **容器**: Docker
- **基础镜像**: Alpine Linux 3.19

## 部署说明

### 前置要求

- Docker已安装并运行
- 本地已存在以下Docker镜像（不要从网络拉取）:
  - nginx:latest
  - golang:1.24-alpine
  - alpine:3.19
  - mysql:latest

### 快速部署

使用提供的部署脚本：

```bash
chmod +x deploy.sh
./deploy.sh
```

### 手动部署

#### 1. 构建后端镜像

```bash
docker build -f Dockerfile.final -t ghh-server .
```

#### 2. 启动MySQL容器

```bash
docker run -d --name mysql-ghh \
  -p 3306:3306 \
  -e MYSQL_ROOT_PASSWORD=rootpassword \
  -e MYSQL_DATABASE=github_hub \
  -v $(pwd)/mysql-data:/var/lib/mysql \
  mysql:latest
```

#### 3. 初始化数据库

```bash
docker exec mysql-ghh mysql -uroot -prootpassword < scripts/init-mysql.sql
```

#### 4. 启动后端容器

```bash
docker run -d --name ghh-server \
  -p 5001:5001 \
  --link mysql-ghh:mysql \
  ghh-server \
  /app/quality-server -addr :5001 -db "root:rootpassword@tcp(mysql:3306)/github_hub?parseTime=true"
```

#### 5. 启动前端容器

```bash
docker run -d --name ghh-frontend \
  -p 80:80 \
  ghh-frontend
```

## API接口

### Webhook端点

- `POST /webhook` - 接收GitHub Webhook事件

### 事件管理

- `GET /api/events` - 获取事件列表
- `GET /api/events/:id` - 获取事件详情
- `DELETE /api/events` - 删除所有事件

### 质量检查

- `GET /api/events/:eventID/quality-checks` - 获取事件的质量检查列表
- `PUT /api/quality-checks/:id` - 更新质量检查状态

### 其他接口

- `GET /api/repositories` - 获取仓库列表
- `GET /api/status` - 获取系统状态
- `POST /api/login` - 用户登录
- `POST /api/logout` - 用户登出
- `GET /api/check-login` - 检查登录状态

## 事件过滤规则

### Push事件

只处理main分支的Push事件，其他分支的Push事件会被忽略。

### Pull Request事件

只处理从非main分支合并到main分支的Pull Request事件，其他PR事件会被忽略。

## 数据库结构

### github_events表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INT | 主键ID |
| event_id | VARCHAR(36) | 事件唯一标识 |
| event_type | VARCHAR(50) | 事件类型 |
| event_status | VARCHAR(50) | 事件状态 |
| repository | VARCHAR(255) | 仓库名称 |
| branch | VARCHAR(255) | 分支名称 |
| target_branch | VARCHAR(255) | 目标分支 |
| commit_sha | VARCHAR(255) | 提交SHA |
| pr_number | INT | PR编号 |
| action | VARCHAR(50) | 操作类型 |
| pusher | VARCHAR(255) | 推送者 |
| author | VARCHAR(255) | 作者 |
| payload | JSON | 事件载荷 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |
| processed_at | TIMESTAMP | 处理时间 |

### pr_quality_checks表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INT | 主键ID |
| github_event_id | VARCHAR(36) | 关联的事件ID |
| check_type | VARCHAR(50) | 检查类型 |
| check_status | VARCHAR(50) | 检查状态 |
| stage | VARCHAR(50) | 检查阶段 |
| stage_order | INT | 阶段顺序 |
| check_order | INT | 检查顺序 |
| started_at | TIMESTAMP | 开始时间 |
| completed_at | TIMESTAMP | 完成时间 |
| duration_seconds | DOUBLE | 持续时间（秒） |
| error_message | TEXT | 错误信息 |
| output | TEXT | 输出信息 |
| retry_count | INT | 重试次数 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

## 开发指南

### 项目结构

```
github-hub/
├── cmd/
│   └── quality-server/      # 后端服务入口
├── internal/
│   ├── quality/
│   │   ├── api/             # API处理
│   │   ├── handlers/        # 事件处理器
│   │   ├── models/          # 数据模型
│   │   └── storage/         # 存储层
│   └── storage/             # 通用存储接口
├── scripts/
│   └── init-mysql.sql       # 数据库初始化脚本
├── Dockerfile.final          # 后端Dockerfile
├── Dockerfile_frontend       # 前端Dockerfile
├── deploy.sh               # 部署脚本
└── README.md               # 项目文档
```

### 本地开发

1. 安装Go 1.24
2. 安装MySQL 9.6
3. 初始化数据库: `mysql -uroot -p < scripts/init-mysql.sql`
4. 运行服务: `go run cmd/quality-server/main.go -db "root:password@tcp(localhost:3306)/github_hub?parseTime=true"`

### 测试

发送测试PR事件：

```bash
curl -X POST http://localhost:5001/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -d @test-pr-event.json
```

## 运维管理

### 查看容器状态

```bash
docker ps
```

### 查看日志

```bash
docker logs ghh-server
docker logs ghh-frontend
docker logs mysql-ghh
```

### 停止服务

```bash
docker stop ghh-server ghh-frontend mysql-ghh
```

### 删除容器

```bash
docker rm ghh-server ghh-frontend mysql-ghh
```

### 备份数据

```bash
docker exec mysql-ghh mysqldump -uroot -prootpassword github_hub > backup.sql
```

### 恢复数据

```bash
docker exec -i mysql-ghh mysql -uroot -prootpassword github_hub < backup.sql
```

## 故障排查

### 容器无法启动

1. 检查端口是否被占用: `netstat -ano | findstr :5001`
2. 检查容器是否已存在: `docker ps -a`
3. 查看容器日志: `docker logs <container_name>`

### 数据库连接失败

1. 检查MySQL容器是否运行: `docker ps | grep mysql`
2. 检查网络连接: `docker exec ghh-server ping mysql`
3. 验证数据库凭据

### Webhook事件未处理

1. 查看服务器日志: `docker logs ghh-server`
2. 检查事件是否符合过滤规则
3. 验证GitHub Webhook配置

## 许可证

MIT License

## 联系方式

如有问题或建议，请联系项目维护者。