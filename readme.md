# Cora
English Version: ![readme_en.md](readme_en.md)

**Cora**（Community Collaboration）是统一的开源社区服务命令行工具。通过单一二进制文件访问论坛、邮件列表、会议、Issue 追踪等社区服务，命令由各后端服务发布的 OpenAPI Spec 动态驱动生成。
![Cora](assets/img/cora.png)
## 项目简介

`cora` 面向每天需要与多个社区服务交互的开源开发者。无需在各种工具和 Web 页面之间来回切换，所有服务统一使用 `cora <服务> <资源> <操作>` 的命令结构。

**核心特点：**

- **零代码扩展** — 接入新的后端服务只需在配置文件中添加一条记录，无需修改 CLI 代码。
- **OpenAPI 驱动** — 命令在运行时根据各服务的 OpenAPI 3.0 Spec 动态生成。
- **Spec 本地缓存** — Spec 缓存到本地（默认 24 小时有效），冷启动无需网络请求，延迟 < 200ms。
- **脚本友好** — stdout/stderr 分离、语义化退出码、`--format json` 输出可直接 pipe 给 `jq`。

## 命令结构

```
cora <服务> <资源> <操作> [参数]
```

| 层级 | 示例 | 来源 |
|------|------|------|
| `cora` | — | 二进制入口 |
| `<服务>` | `forum`、`mail`、`issue` | 配置文件 |
| `<资源>` | `posts`、`topics`、`threads` | OpenAPI `tags[0]` |
| `<操作>` | `list`、`get`、`create`、`delete` | OpenAPI `operationId` |

## 使用示例

```bash
# 列出论坛最新帖子
cora forum posts list

# 获取指定帖子
cora forum posts get --id 42

# 预览创建帖子的 HTTP 请求（不实际发送）
cora forum posts create --title "Release v1.2.0" --raw "正文内容" --dry-run

# 创建帖子
cora forum posts create --title "Release v1.2.0" --raw "正文内容"

# 以 JSON 格式输出并通过 jq 过滤
cora forum posts list --format json | jq '.[].username'

# 强制刷新 OpenAPI Spec 缓存
cora forum posts list --refresh-spec

# 手动刷新指定服务的 Spec 缓存
cora spec refresh forum
```

### 全局参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--format` | `table` | 输出格式：`table` 或 `json` |
| `--dry-run` | `false` | 打印 HTTP 请求详情，不实际发送 |
| `--refresh-spec` | `false` | 跳过缓存，重新拉取服务 Spec |

## 安装

### 从源码构建

**环境要求：** Go 1.22+、make

```bash
git clone https://github.com/cncf/cora.git
cd cora
make build
mv cora /usr/local/bin/
```

### 使用 Docker

```bash
# 构建镜像
make docker-build

# 运行（挂载本地配置目录）
docker run --rm \
  -v ~/.config/cora:/root/.config/cora:ro \
  cora:latest forum posts list
```

或使用 `make docker-run`：

```bash
make docker-run ARGS="forum posts list"
```

## 配置

默认读取 `~/.config/cora/config.yaml`。可通过环境变量 `CORA_CONFIG` 指定其他路径。

### 初始化配置

```bash
mkdir -p ~/.config/cora
cp config.example.yaml ~/.config/cora/config.yaml
# 编辑文件，填写实际值
```

### 配置文件说明

```yaml
services:
  forum:
    # spec_url：服务 OpenAPI Spec 的地址或本地路径。
    # 支持：http://、https://、file:// 或裸文件路径。
    spec_url: assets/openapi/forum/openapi.json

    # base_url：API 根地址，Spec 中的路径会拼接到此地址后。
    base_url: https://forum.example.org

    auth:
      discourse:
        api_key: "你的 API Key"
        api_username: "你的用户名"

  # 按需添加更多服务，无需修改 CLI 代码。
  # mail:
  #   spec_url: https://lists.example.org/openapi.yaml
  #   base_url: https://lists.example.org

# 全局 Spec 缓存配置（可选，括号内为默认值）。
spec_cache:
  ttl: 24h                      # 缓存有效期
  dir: ~/.config/cora/cache     # 缓存存储目录
```

### Spec 加载策略

| 优先级 | 条件 | 行为 |
|--------|------|------|
| 1（最快） | 缓存存在且未过期 | 直接读本地文件，不发起网络请求 |
| 2 | 缓存不存在或已过期 | 从 `spec_url` 拉取，写入缓存 |
| 3 | 拉取失败，存在旧缓存 | 使用旧缓存，stderr 输出警告 |
| 4（报错） | 拉取失败且无缓存 | 退出码 4，提示检查网络和配置 |

使用 `--refresh-spec` 可强制跳过缓存重新拉取。

## 本地开发

### 前置依赖

| 工具 | 版本要求 | 安装 |
|------|----------|------|
| Go | >= 1.22 | `brew install go` |
| make | 任意 | macOS 预装 |
| golangci-lint | >= 1.57 | `brew install golangci-lint` |
| Docker | >= 24.0 | [官网下载](https://www.docker.com) |

### 常用命令

```bash
make build          # 编译二进制（输出：./cora）
make build-prod     # 生产构建（CGO 禁用，去除调试信息）
make test           # 运行全量测试（含竞态检测）
make test-unit      # 仅运行短测试（跳过集成测试）
make test-cover     # 生成 HTML 覆盖率报告（coverage.html）
make lint           # 运行 golangci-lint
make fmt            # 格式化代码（gofmt + goimports）
make tidy           # 整理依赖（go mod tidy）
make clean          # 清理构建产物
```

### 从源码运行

```bash
# 直接运行（无需先构建）
go run ./cmd/cora -- forum posts list

# 构建后运行
make build && ./cora forum posts list
```

### 使用本地 OpenAPI Spec 文件

```yaml
services:
  forum:
    spec_url: assets/openapi/forum/openapi.json   # 相对路径（推荐）
    # spec_url: file:///path/to/openapi.json      # 绝对路径也支持
    base_url: http://localhost:3000
    auth:
      discourse:
        api_key: "dev-api-key"
        api_username: "system"
```

### 测试

测试遵循规范（`testing.md`）的金字塔结构：单元测试为主，覆盖核心逻辑。

```bash
# 全量测试（含竞态检测器）
make test

# 查看覆盖率
make test-cover-text
```

各包测试位置：

| 测试文件 | 覆盖范围 |
|----------|----------|
| `pkg/errs/errors_test.go` | 错误类型、退出码、构造函数 |
| `internal/spec/cache_test.go` | 缓存读写、原子写入、TTL |
| `internal/spec/loader_test.go` | 三段式加载、HTTP、本地文件、降级策略 |
| `internal/builder/command_test.go` | 资源名推导、动词解析、Flag 名转换 |
| `internal/output/formatter_test.go` | JSON/Table 输出、终端安全、数据提取 |

### 项目目录结构

```
cora/
├── cmd/cora/main.go                  # 入口，两阶段命令加载
├── internal/
│   ├── builder/
│   │   ├── command.go                # OpenAPI Spec → Cobra 命令树
│   │   └── command_test.go
│   ├── config/config.go              # 配置加载与结构体定义
│   ├── executor/executor.go          # HTTP 请求执行
│   ├── output/
│   │   ├── formatter.go              # Table / JSON 输出格式化
│   │   └── formatter_test.go
│   ├── registry/registry.go          # 服务注册表
│   ├── spec/
│   │   ├── loader.go                 # 三段式 Spec 加载
│   │   ├── loader_test.go
│   │   ├── cache.go                  # 缓存读写（原子写入）
│   │   └── cache_test.go
│   └── auth/resolver.go              # 鉴权头注入
├── pkg/errs/
│   ├── errors.go                     # 错误类型与退出码定义
│   └── errors_test.go
├── assets/openapi/                   # 本地 Spec 文件（供本地开发使用）
├── spec/                             # 架构设计文档
├── Makefile
├── Dockerfile
└── config.example.yaml
```

## 接入新服务

1. 确保后端服务在固定地址发布了 OpenAPI 3.0 Spec。
2. 在 `~/.config/cora/config.yaml` 中添加配置：

   ```yaml
   services:
     myservice:
       spec_url: https://myservice.example.org/openapi.yaml
       base_url: https://myservice.example.org
   ```

3. 执行任意命令，Spec 会自动拉取并缓存：

   ```bash
   cora myservice --help
   ```

### 后端服务接入要求

- 使用 **OpenAPI 3.0** 规范（不支持 Swagger 2.0）
- 为每个操作声明 `tags`，第一个 tag 即为 CLI 的 `<资源>` 层命令名
- `operationId` 使用已知动词前缀：`list`、`get`、`create`、`update`、`delete`、`patch`
- 通过 `security` 字段按操作声明鉴权需求（缺失或为空表示无需鉴权）

可选的 `x-cli-*` 扩展字段可增强 CLI 体验：

```yaml
x-cli-examples:
  - "cora myservice widgets list --active"
x-cli-flags: [active, limit, cursor]
```

## 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 成功 |
| 1 | API 错误（后端返回 4xx/5xx） |
| 2 | 鉴权错误（未登录或凭证无效） |
| 3 | 参数校验错误 |
| 4 | Spec 加载失败 |
| 5 | 配置错误（服务未配置或配置文件损坏） |
| 127 | 未分类错误 |

## 架构文档

完整架构设计（含框架选型、OpenAPI 驱动命令生成、鉴权策略、Spec 缓存等 ADR）请参阅 [`spec/architecture-design.md`](spec/architecture-design.md)。

## 贡献

欢迎贡献代码。提交较大改动前请先开 Issue 说明意图。
