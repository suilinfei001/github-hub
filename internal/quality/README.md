# Quality Engine Module

Quality Engine 是一个 GitHub 事件质量检查引擎，用于监控和检查 GitHub 仓库的事件质量，包括 PR 和 Push 事件。

## 功能特性

- **GitHub 事件处理**：支持处理 PR 和 Push 事件
- **事件过滤**：根据分支和事件类型进行过滤
- **质量检查流水线**：实现了完整的质量检查流水线，包括编译、代码 lint、安全扫描、单元测试、部署和专项测试
- **数据存储**：使用文件系统存储事件和质量检查数据，无需数据库
- **API 接口**：提供了完整的 RESTful API 接口，用于管理事件和质量检查
- **Mock 数据**：提供了 Mock 事件功能，方便测试和开发
- **前端界面**：集成了前端代码，提供了友好的 Web 界面

## 快速开始

### 本地运行

1. 启动质量引擎服务器：

   ```bash
   go run ./cmd/quality-server
   ```

2. 服务器默认在端口 5000 上运行，可以通过 `-addr` 参数指定端口：

   ```bash
   go run ./cmd/quality-server -addr :5001
   ```

3. 服务器默认在当前目录存储数据，可以通过 `-root` 参数指定数据存储目录：

   ```bash
   go run ./cmd/quality-server -root /path/to/data
   ```

### Docker 运行

1. 构建 Docker 镜像：

   ```bash
   docker build -t github-hub:latest .
   ```

2. 运行 Docker 容器：

   ```bash
   docker run -p 8080:8080 -p 5000:5000 -v /path/to/data:/data github-hub:latest
   ```

3. 运行质量引擎服务器：

   ```bash
   docker exec -it <container-id> quality-server
   ```

## API 接口

### Webhook 端点

- **POST /webhook**：接收 GitHub Webhook 事件

### 事件管理

- **GET /api/events**：获取事件列表，支持过滤
- **DELETE /api/events**：删除所有事件
- **GET /api/events/{id}**：获取事件详情

### 质量检查管理

- **GET /api/events/{id}/quality-checks**：获取事件的质量检查列表
- **PUT /api/quality-checks/{id}**：更新质量检查状态

### Mock 数据

- **GET /api/mock/events**：获取 Mock 事件模板
- **POST /api/mock/simulate/{event-type}**：模拟事件

### 认证

- **POST /api/login**：登录
- **POST /api/logout**：登出
- **GET /api/check-login**：检查登录状态

### 系统状态

- **GET /api/status**：获取系统状态

## 前端界面

服务器启动后，可以通过浏览器访问 `http://localhost:5000` 打开前端界面，用于管理事件和质量检查。

## 事件过滤规则

- **Push 事件**：只处理 main 分支的 push 事件
- **PR 事件**：只处理非 main 分支合入 main 分支的 PR 事件

## 质量检查流水线

1. **基础 CI 阶段**：
   - 编译检查
   - 代码 lint 检查
   - 安全扫描
   - 单元测试

2. **部署阶段**：
   - 部署检查

3. **专项测试阶段**：
   - API 测试
   - 模块 E2E 测试
   - Agent E2E 测试
   - AI E2E 测试

## 数据存储

数据存储在文件系统中，默认存储在 `quality-engine` 目录下：

- `quality-engine/events/`：存储事件数据
- `quality-engine/quality_checks/`：存储质量检查数据
- `quality-engine/static/`：存储前端静态文件

## 配置

### 命令行参数

- `-addr`：服务器监听地址，默认 `:5000`
- `-root`：数据存储根目录，默认 `.`

### 环境变量

- `GITHUB_TOKEN`：GitHub API 访问令牌（可选）

## 开发指南

### 目录结构

```
internal/quality/
├── api/          # API 服务器和路由
├── handlers/     # 事件处理器
├── models/       # 数据模型和枚举
├── storage/      # 数据存储
└── static/       # 前端静态文件
```

### 核心组件

1. **Server**：质量引擎服务器，处理 HTTP 请求
2. **PRHandler**：PR 事件处理器
3. **PushHandler**：Push 事件处理器
4. **FileStorage**：文件系统存储实现
5. **GitHubEvent**：GitHub 事件模型
6. **PRQualityCheck**：PR 质量检查模型

### 测试

使用以下命令测试核心功能：

1. 启动服务器：
   ```bash
   go run ./cmd/quality-server
   ```

2. 模拟 PR 事件：
   ```bash
   curl -X POST http://localhost:5000/api/mock/simulate/pull_request -H "Content-Type: application/json" -d '{"repository": "owner/repo", "pr_number": 123, "pr_title": "Test PR", "source_branch": "feature/test", "target_branch": "main", "pr_author": "developer"}'
   ```

3. 获取事件列表：
   ```bash
   curl http://localhost:5000/api/events
   ```

4. 更新质量检查状态：
   ```bash
   curl -X PUT http://localhost:5000/api/quality-checks/1 -H "Content-Type: application/json" -d '{"status": "passed", "output": "Compilation successful"}'
   ```

## 部署指南

### 本地部署

1. 编译服务器：
   ```bash
   go build -o quality-server ./cmd/quality-server
   ```

2. 运行服务器：
   ```bash
   ./quality-server
   ```

### Docker 部署

1. 构建 Docker 镜像：
   ```bash
   docker build -t github-hub:latest .
   ```

2. 运行 Docker 容器：
   ```bash
   docker run -p 8080:8080 -p 5000:5000 -v /path/to/data:/data github-hub:latest
   ```

3. 在容器中运行质量引擎服务器：
   ```bash
   docker exec -it <container-id> quality-server
   ```

### Kubernetes 部署

使用以下 YAML 文件部署到 Kubernetes：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: github-hub
  labels:
    app: github-hub
spec:
  replicas: 1
  selector:
    matchLabels:
      app: github-hub
  template:
    metadata:
      labels:
        app: github-hub
    spec:
      containers:
      - name: github-hub
        image: github-hub:latest
        ports:
        - containerPort: 8080
        - containerPort: 5000
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: github-hub-data
---
apiVersion: v1
kind: Service
metadata:
  name: github-hub
spec:
  selector:
    app: github-hub
  ports:
  - port: 8080
    targetPort: 8080
  - port: 5000
    targetPort: 5000
  type: LoadBalancer
```

## 监控和日志

### 日志

服务器日志输出到标准输出，可以通过以下方式查看：

1. 本地运行：直接在终端查看
2. Docker 运行：`docker logs <container-id>`
3. Kubernetes 运行：`kubectl logs <pod-name>`

### 系统状态

通过以下 API 端点获取系统状态：

```bash
curl http://localhost:5000/api/status
```

## 故障排查

### 常见问题

1. **端口占用**：如果端口 5000 已被占用，可以使用 `-addr` 参数指定其他端口
2. **数据存储**：确保数据存储目录有写入权限
3. **GitHub Webhook**：确保 Webhook 配置正确，包括事件类型和 URL

### 调试模式

使用以下命令启动服务器，获取更详细的日志：

```bash
GODEBUG=1 go run ./cmd/quality-server
```

## 贡献指南

欢迎贡献代码和功能！请按照以下步骤进行：

1. Fork 仓库
2. 创建分支
3. 提交代码
4. 创建 PR

## 许可证

本项目使用 MIT 许可证，详情请查看根目录下的 LICENSE 文件。
