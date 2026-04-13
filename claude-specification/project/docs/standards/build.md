# 构建规范

> 本文件定义项目的编译、构建、运行和发布规范。
> 每个项目必须填写本文件，这是项目层规范的核心文件之一。

---

## 1. 开发环境

### 1.1 前置依赖

> [填写] 列出所有必须安装的工具和版本，例如：

| 工具          | 版本要求 | 安装说明                       |
| ------------- | -------- | ------------------------------ |
| Go            | >= 1.22  | `brew install go`              |
| Docker        | >= 24.0  | [官网下载](https://docker.com) |
| make          | 任意     | macOS 预装                     |
| golangci-lint | >= 1.57  | `brew install golangci-lint`   |

### 1.2 初始化步骤

> [填写] 从零开始的完整初始化步骤：

```bash
# 1. 安装依赖
make deps

# 2. 配置本地环境变量
cp .env.example .env.local
# 编辑 .env.local 填写必要配置（见下方环境变量说明）

# 3. 启动基础设施（数据库、缓存等）
make infra-up

# 4. 执行数据库迁移
make migrate

# 5. 启动开发服务
make dev
```

---

## 2. 环境变量

### 2.1 必填变量

> [填写] 所有必填环境变量及说明：

| 变量名         | 说明                               | 示例值                                   |
| -------------- | ---------------------------------- | ---------------------------------------- |
| `DATABASE_URL` | 数据库连接串                       | `postgres://user:pass@localhost:5432/db` |
| `REDIS_URL`    | Redis 连接串                       | `redis://localhost:6379`                 |
| `JWT_SECRET`   | JWT 签名密钥（本地随机字符串即可） | `dev-secret-32chars`                     |
| `APP_ENV`      | 运行环境                           | `development` / `production`             |

### 2.2 可选变量

> [填写] 可选环境变量及默认值：

| 变量名       | 说明     | 默认值 |
| ------------ | -------- | ------ |
| `APP_PORT`   | 服务端口 | `8080` |
| `LOG_LEVEL`  | 日志级别 | `info` |
| `LOG_FORMAT` | 日志格式 | `json` |

---

## 3. 常用命令

### 3.1 开发命令

> [填写] Makefile 主要目标说明：

```bash
make dev          # 启动开发服务（支持热重载）
make test         # 运行所有测试
make test-unit    # 仅运行单元测试
make test-int     # 仅运行集成测试（需要 infra 运行）
make lint         # 代码风格检查
make fmt          # 代码格式化
make build        # 编译二进制
make clean        # 清理构建产物
```

### 3.2 数据库命令

```bash
make migrate         # 执行所有待执行的迁移
make migrate-down    # 回滚最近一次迁移
make migrate-status  # 查看迁移状态
make seed            # 插入测试种子数据（仅开发环境）
```

### 3.3 代码生成命令

> [填写] 如有代码生成，说明触发条件和命令：

```bash
make proto        # 从 .proto 文件生成 Go 代码（修改 proto 后执行）
make mock         # 生成 mock 文件（修改 interface 后执行）
make swagger      # 生成 OpenAPI 文档（修改 API 注释后执行）
```

---

## 4. 构建

### 4.1 编译

> [填写] 构建参数和产物说明，例如：

```bash
# 本地构建
make build
# 产物：./bin/<service-name>

# 生产构建（含版本信息注入）
make build-prod VERSION=$(git describe --tags)
```

### 4.2 版本信息

> [填写] 版本信息注入方式，例如：
>
> - 版本号通过 Git Tag 获取（语义化版本 vX.Y.Z）
> - 编译时注入：`go build -ldflags "-X main.Version=$(VERSION)"`

### 4.3 Docker 镜像

> [填写] 镜像构建和推送规范，例如：

```bash
# 构建镜像
make docker-build VERSION=v1.2.3

# 镜像命名规范
<registry>/<team>/<service>:<version>
# 示例：registry.example.com/platform/user-service:v1.2.3
```

---

## 5. CI/CD 流水线

### 5.1 CI 触发条件

> [填写] 例如：

| 触发事件         | 执行内容                                    |
| ---------------- | ------------------------------------------- |
| PR 提交          | lint + 单元测试 + 覆盖率检查                |
| merge to main    | 全量测试 + 构建 + 推送镜像（dev tag）       |
| 打 Tag（vX.Y.Z） | 构建 + 推送镜像（版本 tag）+ 部署到 staging |
| 手动触发         | 部署到指定环境                              |

### 5.2 CI 门禁

> [填写] 必须通过才能合并的检查，例如：
>
> - lint 检查通过
> - 所有测试通过
> - 代码覆盖率 >= 80%
> - 无高危安全漏洞（trivy 扫描）

### 5.3 部署流程

> [填写] 部署流程和注意事项，例如：
>
> - 部署顺序：staging → 灰度（10%）→ 全量
> - 数据库迁移在部署前自动执行（Job 形式）
> - 回滚方式：切换 Kubernetes Deployment 镜像版本

---

## 6. 本地基础设施（Docker Compose）

> [填写] `docker-compose.yml` 中各服务说明，例如：

```yaml
# docker-compose.yml 服务说明
services:
  postgres: # 数据库，端口 5432
  redis: # 缓存，端口 6379
  kafka: # 消息队列，端口 9092（如有）
  minio: # 对象存储，端口 9000（如有）
```

---

## 7. 故障排查

> [填写] 常见构建/启动问题及解决方法，例如：

**问题：`make dev` 启动报错 `connection refused (database)`**

- 原因：数据库未启动
- 解决：`make infra-up`，等待 15 秒后重试

**问题：测试失败，报 `port already in use`**

- 原因：测试使用固定端口，前一次测试未正常退出
- 解决：`lsof -i :<port> | grep LISTEN`，kill 对应进程

> 更多运维问题见 `docs/runbook.md`
