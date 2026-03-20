# TSPeek

TeamSpeak 服务器实时监控仪表板。通过 ServerQuery 协议连接 TeamSpeak 3 服务器，周期性获取服务器信息、频道列表和在线客户端数据，并通过 Web UI 实时展示。

## 功能特性

- 实时监控服务器状态（名称、版本、在线人数等）
- 频道树形结构展示，包含各频道内的客户端信息
- 客户端详细状态显示（麦克风、扬声器、离开状态等）
- 基于 Server-Sent Events (SSE) 的实时数据推送
- 单二进制部署，前端资源内嵌
- 支持健康检查端点（`/healthz`、`/readyz`）

## 技术栈

- **后端**: Go 1.22
- **前端**: React 19 + TypeScript + Fluent UI
- **构建**: Vite 8 + pnpm
- **容器化**: Docker 多阶段构建

## 本地构建

### 前置要求

- Go >= 1.22
- Node.js >= 22
- pnpm

### 构建步骤

```bash
# 1. 构建前端
cd web
pnpm install
pnpm build

# 2. 复制前端产物到 embed 目录
cp -r dist ../internal/api/dist

# 3. 构建 Go 二进制
cd ..
go build -o tspeek ./cmd/tspeek
```

## 运行

```bash
# 复制并编辑配置文件
cp config.example.yaml config.yaml

# 启动
./tspeek -config config.yaml
```

也可以通过环境变量指定配置文件路径：

```bash
TSPEEK_CONFIG=/path/to/config.yaml ./tspeek
```

启动后访问 `http://localhost:8080` 查看仪表板。

## 配置说明

参考 `config.example.yaml`：

```yaml
listen_address: ":8080"
log_level: "info"              # debug / info / warn / error

http:
  read_timeout: "5s"
  write_timeout: "30s"
  idle_timeout: "60s"

dashboard:
  refresh_interval: "5s"       # 数据刷新间隔
  show_query_clients: false    # 是否显示 ServerQuery 客户端

serverquery:
  host: "example.com"          # TeamSpeak 服务器地址
  query_port: 10011            # ServerQuery 端口
  username: "serverquery_user"
  password: "change-me"
  server_port: 9987            # TeamSpeak 服务端口
  # sid: 1                     # 服务器 ID（可选）
  dial_timeout: "5s"
  command_timeout: "10s"
```

## Docker 部署

### 使用预构建镜像

```bash
docker run -d \
  --name tspeek \
  -p 8080:8080 \
  -v /path/to/config.yaml:/config/config.yaml \
  kougami132/tspeek
```

### 本地构建镜像

```bash
docker build -t tspeek .
docker run -d \
  --name tspeek \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/config/config.yaml \
  tspeek
```

### Docker Compose

创建 `docker-compose.yml`：

```yaml
services:
  tspeek:
    image: kougami132/tspeek
    container_name: tspeek
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/config/config.yaml:ro
```

启动：

```bash
docker compose up -d
```
