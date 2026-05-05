# Cora
[English Version](readme_en.md)

**Cora**（Community Collaboration）是统一的开源社区服务命令行工具。通过单一二进制文件访问论坛、邮件列表、会议、Issue CICD等社区服务，命令由各后端服务发布的 OpenAPI Spec 动态驱动生成。

![Cora](assets/img/cora.png)

## 项目简介

`cora` 面向每天需要与多个社区服务交互的开源开发者。无需在各种工具和 Web 页面之间来回切换，所有服务统一使用 `cora <服务> <资源> <操作>` 的命令结构。

**核心特点：**

- **零代码扩展** — 接入新的后端服务只需在配置文件中添加一条记录，无需修改 CLI 代码。
- **OpenAPI 驱动** — 命令在运行时根据各服务的 OpenAPI 3.0 Spec 动态生成。
- **Spec 本地缓存** — Spec 缓存到本地（默认 24 小时有效），冷启动无需网络请求，延迟 < 200ms。
- **输出结果可定制** — 通过声明式配置定制每个操作的输出字段和展示方式；`--format json/yaml` 可直接输出完整原始数据，适合脚本和 Agent 使用。
- **脚本友好** — stdout/stderr 分离、语义化退出码、`--format json` 输出可直接 pipe 给 `jq`。

## 已支持服务

| 服务                                   | 命令名        | Spec 来源  | 鉴权方式                              |
|--------------------------------------|------------|----------|-----------------------------------|
| [ GitCode ](https://gitcode.com)     | `gitcode`  | 内置嵌入     | 个人访问令牌（`?access_token=`), 统一认证待补充 |
| [ GitHub ](https://github.com)       | `github`   | 内置嵌入     | PAT / Fine-grained Token（`Authorization: Bearer …`） |
| [ Etherpad ](https://etherpad.org)   | `etherpad` | 内置嵌入     | API Key（`?apikey=`）, 统一认证待补充      |
| [ Jenkins ](https://www.jenkins.io)  | `jenkins`  | 内置嵌入     | HTTP Basic Auth（`base64(username:api_token)`） |
| [ Forum ](https://www.discourse.org) | `forum`    | spec_url | API Key + 用户名（请求头）,  统一认证待补充      |

## 命令结构

```
cora <服务> <资源> <操作> [参数]
```

| 层级     | 示例                             | 来源                    |
|--------|--------------------------------|-----------------------|
| `cora` | —                              | 二进制入口                 |
| `<服务>` | `gitcode`、`forum`、`etherpad`   | OpenAPI               |
| `<资源>` | `issues`、`posts`、`topics`      | OpenAPI `tags[0]`     |
| `<操作>` | `list`、`get`、`create`、`delete` | OpenAPI `operationId` |

## 使用示例

### GitCode

```bash
# 列出仓库列表
cora gitcode repos list --owner my-org

# 获取仓库信息
cora gitcode repos get --owner my-org --repo my-repo

# 列出 Issue
cora gitcode issues list --owner my-org --repo my-repo --state open

# 获取单个 Issue（表格展示）
cora gitcode issues get --owner my-org --repo my-repo --number 1367

# 以 JSON 格式输出（原始完整数据，可 pipe 给 jq）
cora gitcode issues get --owner my-org --repo my-repo --number 1367 --format json | jq '.title'

# 以 YAML 格式输出
cora gitcode issues list --owner my-org --repo my-repo --format yaml

# 预览请求内容（不实际发送）
cora gitcode issues create --owner my-org --repo my-repo --title "test" --dry-run
```

### GitHub

```bash
# 获取仓库信息
cora github repos get --owner cncf --repo cora

# 列出仓库 Issue
cora github issues list --owner cncf --repo cora --state open

# 获取单个 Issue
cora github issues get --owner cncf --repo cora --issue-number 1

# 列出 Pull Request
cora github pulls list --owner cncf --repo cora --state open

# JSON 输出 + jq 提取
cora github issues get --owner cncf --repo cora --issue-number 1 --format json | jq '.title'
```

### Forum（Discourse）

```bash
# 列出论坛最新帖子
cora forum posts list

# 获取指定帖子
cora forum posts get --id 42

# 创建帖子
cora forum posts create --title "Release v1.2.0" --raw "正文内容"

# 以 JSON 格式输出并通过 jq 过滤
cora forum posts list --format json | jq '.[].username'

# 强制刷新 OpenAPI Spec 缓存
cora forum posts list --refresh-spec
```

### Etherpad

```bash
# 列出所有 pad
cora etherpad pads list

# 获取 pad 内容
cora etherpad pads get-text --pad-id my-pad

# 创建新 pad
cora etherpad pads create-pad --pad-id new-pad
```

### Jenkins

```bash
# 列出所有 Job
cora jenkins jobs list

# 获取单个 Job 详情
cora jenkins jobs get --name my-job

# 触发构建
cora jenkins jobs build --name my-job

# 启用 / 禁用 Job
cora jenkins jobs enable-job --name my-job
cora jenkins jobs disable-job --name my-job

# 删除 Job
cora jenkins jobs delete --name my-job

# 获取构建详情
cora jenkins builds get --name my-job --number 1

# 查看队列
cora jenkins queue list

# JSON 格式输出
cora jenkins jobs list --format json | jq '.jobs[].name'
```

### 全局参数

| 参数               | 默认值     | 说明                           |
|------------------|---------|------------------------------|
| `--format`       | `table` | 输出格式：`table`、`json` 或 `yaml` |
| `--dry-run`      | `false` | 打印 HTTP 请求详情，不实际发送           |
| `--refresh-spec` | `false` | 跳过缓存，重新拉取服务 Spec             |
| `--verbose`      | `false` | 输出详细调试日志（INFO + DEBUG 级别）    |

## 输出结果定制

### 格式说明

`--format` 控制全局输出格式，对所有子命令生效：

| 值           | 行为                                    |
|-------------|---------------------------------------|
| `table`（默认） | 应用 View 定义展示格式化表格；无 View 时自动 fallback |
| `json`      | 跳过所有 View，完整响应体 pretty-print 为 JSON   |
| `yaml`      | 跳过所有 View，完整响应体转换为 YAML               |

**`--format json/yaml` 永远输出完整、未经过滤的原始响应**，适合脚本和 Agent 使用。View 系统只在 `--format table` 时生效。

### View 系统

cora 内置了常用操作的 View 定义（字段选取、格式化），同时支持通过 `~/.config/cora/views.yaml` 用户自定义覆盖或新增。

**内置 View 覆盖的操作：**

| 服务       | 操作            | 展示模式         |
|----------|---------------|--------------|
| gitcode  | `issues get`  | 竖式 KV 表（单对象） |
| gitcode  | `issues list` | 横向表格（列表）     |
| gitcode  | `repos get`   | 竖式 KV 表      |
| gitcode  | `repos list`  | 横向表格         |
| gitcode  | `pulls get`   | 竖式 KV 表      |
| gitcode  | `pulls list`  | 横向表格         |
| github   | `issues get`  | 竖式 KV 表      |
| github   | `issues list` | 横向表格         |
| github   | `repos get`   | 竖式 KV 表      |
| github   | `repos list`  | 横向表格         |
| github   | `pulls get`   | 竖式 KV 表      |
| github   | `pulls list`  | 横向表格         |
| github   | `users get`   | 竖式 KV 表      |
| forum    | `topics list` | 横向表格         |
| forum    | `topics get`  | 竖式 KV 表      |
| forum    | `posts list`  | 横向表格         |
| etherpad | `pads list`   | 横向表格         |

### 配置 views.yaml

将 `views.example.yaml` 复制到 `~/.config/cora/views.yaml` 并按需修改：

```bash
mkdir -p ~/.config/cora
cp views.example.yaml ~/.config/cora/views.yaml
```

也可通过环境变量指定其他路径：

```bash
export CORA_VIEWS=/path/to/my-views.yaml
```

或在 `config.yaml` 中配置：

```yaml
views_file: /path/to/my-views.yaml
```

### views.yaml 格式

```yaml
<服务名>:
  <资源>/<操作>:
    root_field: ""        # 可选：响应中包含列表的字段名（空=自动探测）
    columns:
      - field: <dot.path> # 必填：JSON 字段路径，支持点号嵌套（如 user.login）
        label: <string>   # 可选：表头（默认自动 title-case）
        format: <type>    # 可选：text（默认）| json | date | multiline
        truncate: <int>   # 可选：最大字符数，0=不截断
        width: <int>      # 可选：固定列宽（仅列表横向表格有效）
        date_fmt: <string># 可选：Go 时间格式，format=date 时有效
        indent: <bool>    # 可选：format=json 时是否缩进展示
```

**format 类型说明：**

| 值           | 适用场景                  | 渲染逻辑                           |
|-------------|-----------------------|--------------------------------|
| `text`（默认）  | 字符串、数字、布尔             | 转为字符串，按 `truncate` 截断          |
| `json`      | 嵌套对象、数组               | 保留原始 JSON；`indent: true` 时缩进展示 |
| `date`      | ISO 8601 时间戳          | 解析后按 `date_fmt` 重新格式化          |
| `multiline` | 长文本（body、description） | 保留换行符，按 `truncate` 字符数截断       |

### 示例：覆盖内置 View，只显示部分字段

```yaml
# ~/.config/cora/views.yaml
gitcode:
  issues/get:
    columns:
      - field: number
        label: "No."
      - field: title
        label: Title
        truncate: 80
      - field: state
      - field: html_url
        label: URL
      - field: user.login
        label: Author
      - field: created_at
        label: Created
        format: date
```

### 示例：为尚无内置 View 的操作自定义

```yaml
gitcode:
  commits/list:
    columns:
      - field: sha
        label: SHA
        truncate: 8
        width: 10
      - field: commit.message
        label: Message
        truncate: 60
        width: 62
      - field: commit.author.name
        label: Author
        width: 18
      - field: commit.author.date
        label: Date
        format: date
        width: 12
```

用户 View 完全覆盖同键的内置 View（整体替换，不合并列）。

## 安装

### 从源码构建

**环境要求：** Go 1.22+、make

```bash
git clone https://github.com/cncf/cora.git
cd cora
make build
mv bin/cora /usr/local/bin/
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
cp config/config.example.yaml ~/.config/cora/config.yaml
# 编辑文件，填写实际值
```

### 配置文件说明

```yaml
services:
  # ── GitCode（内置 Spec，无需 spec_url）──
  gitcode:
    base_url: https://api.gitcode.com   # 必填，无默认值
    auth:
      gitcode:
        access_token: "你的个人访问令牌"  # GitCode 用户设置 → 个人访问令牌

  # ── GitHub（内置 Spec，无需 spec_url）──
  github:
    base_url: https://api.github.com    # 必填；GHE Server 改为 https://<host>/api/v3
    auth:
      github:
        token: "你的 GitHub PAT"          # https://github.com/settings/tokens

  # ── Etherpad（内置 Spec，无需 spec_url）──
  etherpad:
    base_url: https://your-etherpad-host/api/1.3.0  # 必填，无默认值
    auth:
      etherpad:
        api_key: "你的 Etherpad API Key"

  # ── Jenkins（内置 Spec，无需 spec_url）──
  jenkins:
    base_url: https://jenkins.example.com          # 必填，无默认值
    auth:
      jenkins:
        username: "你的 Jenkins 用户名"
        api_token: "你的 Jenkins API Token"          # JENKINS_URL/user/<you>/configure

  # ── Forum / Discourse（需要 spec_url）──
  forum:
    spec_url: assets/openapi/forum/openapi.json   # 支持 http://、https://、file:// 或裸路径
    base_url: https://forum.example.org            # 必填
    auth:
      discourse:
        api_key: "你的 API Key"
        api_username: "你的用户名"

  # 按需添加更多服务，无需修改 CLI 代码。
  # myservice:
  #   spec_url: https://myservice.example.org/openapi.yaml
  #   base_url: https://myservice.example.org

# 全局 Spec 缓存配置（可选，括号内为默认值）
spec_cache:
  ttl: 24h                      # 缓存有效期
  dir: ~/.config/cora/cache     # 缓存存储目录

# 自定义 views.yaml 路径（可选，默认 ~/.config/cora/views.yaml）
views_file: ~/.config/cora/views.yaml
```

> **注意**：内置服务（gitcode、github、etherpad、jenkins）的 `base_url` 没有硬编码默认值，必须在配置文件中显式声明。

### 环境变量

所有配置项均可通过 `CORA_` 前缀的环境变量覆盖，优先级高于配置文件。

| 环境变量                                               | 对应配置项                                         | 说明                |
|----------------------------------------------------|-----------------------------------------------|-------------------|
| `CORA_CONFIG`                                      | —                                             | 覆盖配置文件路径          |
| `CORA_VIEWS`                                       | —                                             | 覆盖 views.yaml 路径  |
| `CORA_SPEC_CACHE_TTL`                              | `spec_cache.ttl`                              | 缓存有效期（如 `12h`）    |
| `CORA_SPEC_CACHE_DIR`                              | `spec_cache.dir`                              | 缓存目录路径            |
| `CORA_SERVICES_<NAME>_BASE_URL`                    | `services.<name>.base_url`                    | 覆盖指定服务的 API 根地址   |
| `CORA_SERVICES_<NAME>_SPEC_URL`                    | `services.<name>.spec_url`                    | 覆盖指定服务的 Spec 地址   |
| `CORA_SERVICES_GITCODE_AUTH_GITCODE_ACCESS_TOKEN`  | `services.gitcode.auth.gitcode.access_token`  | GitCode 个人访问令牌    |
| `CORA_SERVICES_GITHUB_AUTH_GITHUB_TOKEN`           | `services.github.auth.github.token`           | GitHub PAT / 细粒度令牌 |
| `CORA_SERVICES_ETHERPAD_AUTH_ETHERPAD_API_KEY`     | `services.etherpad.auth.etherpad.api_key`     | Etherpad API Key  |
| `CORA_SERVICES_JENKINS_AUTH_JENKINS_USERNAME`     | `services.jenkins.auth.jenkins.username`     | Jenkins 用户名       |
| `CORA_SERVICES_JENKINS_AUTH_JENKINS_API_TOKEN`    | `services.jenkins.auth.jenkins.api_token`    | Jenkins API Token |
| `CORA_SERVICES_<NAME>_AUTH_DISCOURSE_API_KEY`      | `services.<name>.auth.discourse.api_key`      | Discourse API Key |
| `CORA_SERVICES_<NAME>_AUTH_DISCOURSE_API_USERNAME` | `services.<name>.auth.discourse.api_username` | Discourse 用户名     |

> **注意**：环境变量只能覆盖配置文件中**已存在**的服务条目，无法通过环境变量新增服务。

#### 本地开发：使用 .env 文件

在项目根目录创建 `.env` 文件，cora 启动时会自动加载其中的变量（已存在的系统环境变量不会被覆盖）。`.env` 文件仅供本地开发使用，**不要提交到版本库**。

```bash
cp .env.example .env
# 编辑 .env，填入本地开发的实际值
```

`.env` 示例：

```bash
CORA_SERVICES_FORUM_BASE_URL=http://localhost:3000
CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_KEY=dev-api-key
CORA_SERVICES_FORUM_AUTH_DISCOURSE_API_USERNAME=system
```

### Spec 加载策略

| 优先级   | 条件         | 行为                   |
|-------|------------|----------------------|
| 1（最快） | 缓存存在且未过期   | 直接读本地文件，不发起网络请求      |
| 2     | 缓存不存在或已过期  | 从 `spec_url` 拉取，写入缓存 |
| 3     | 拉取失败，存在旧缓存 | 使用旧缓存，stderr 输出警告    |
| 4（报错） | 拉取失败且无缓存   | 退出码 4，提示检查网络和配置      |

使用 `--refresh-spec` 可强制跳过缓存重新拉取。

## 本地开发

### 前置依赖

| 工具            | 版本要求    | 安装                             |
|---------------|---------|--------------------------------|
| Go            | >= 1.22 | `brew install go`              |
| make          | 任意      | macOS 预装                       |
| golangci-lint | >= 1.57 | `brew install golangci-lint`   |
| Docker        | >= 24.0 | [官网下载](https://www.docker.com) |

### 常用命令

```bash
make build          # 编译二进制（输出：./bin/cora）
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
make build && ./bin/cora forum posts list
```

### 测试

```bash
# 全量测试（含竞态检测器）
make test

# 查看覆盖率
make test-cover-text
```

各包测试位置：

| 测试文件                                | 覆盖范围                            |
|-------------------------------------|---------------------------------|
| `pkg/errs/errors_test.go`           | 错误类型、退出码、构造函数                   |
| `internal/spec/cache_test.go`       | 缓存读写、原子写入、TTL                   |
| `internal/spec/loader_test.go`      | 三段式加载、HTTP、本地文件、降级策略            |
| `internal/builder/command_test.go`  | 资源名推导、动词解析、Flag 名转换             |
| `internal/log/mask_test.go`         | URL 脱敏、请求头脱敏、响应体格式化             |
| `internal/output/formatter_test.go` | JSON/YAML/Table 输出、View 渲染、终端安全 |
| `internal/smoke/loader_test.go`     | YAML 场景加载、默认值、空文件跳过           |
| `internal/smoke/assertion_test.go`  | 10 种断言逻辑验证                         |
| `internal/smoke/runner_test.go`     | 子进程调用、环境变量注入                   |
| `internal/smoke/report_test.go`     | HTML 报告生成                            |

### 项目目录结构

```
cora/
├── cmd/cora/main.go                  # 入口，两阶段命令加载
├── internal/
│   ├── auth/resolver.go              # 鉴权凭证注入
│   ├── builder/
│   │   ├── command.go                # OpenAPI Spec → Cobra 命令树
│   │   └── command_test.go
│   ├── config/config.go              # 配置加载与结构体定义
│   ├── executor/executor.go          # HTTP 请求执行
│   ├── log/
│   │   ├── log.go                    # 分级日志（Error/Warn/Info/Debug）
│   │   └── mask.go                   # URL/Header 脱敏，响应体格式化
│   ├── output/
│   │   ├── formatter.go              # Table / JSON / YAML 输出格式化
│   │   └── formatter_test.go
│   ├── registry/
│   │   ├── registry.go               # 服务注册表
│   │   └── builtin.go                # 内置服务注册（gitcode、github、etherpad、jenkins）
│   ├── spec/
│   │   ├── loader.go                 # 三段式 Spec 加载
│   │   ├── cache.go                  # 缓存读写（原子写入）
│   │   └── *_test.go
│   └── view/
│       ├── view.go                   # ViewColumn / ViewConfig / Registry 类型定义
│       ├── extract.go                # 字段路径提取与值格式化
│       ├── builtin.go                # 内置 View 定义
│       └── loader.go                 # 加载 views.yaml，合并内置 View
├── pkg/errs/
│   ├── errors.go                     # 错误类型与退出码定义
│   └── errors_test.go
├── assets/
│   ├── openapi/                      # 内置服务 OpenAPI Spec 文件
│   └── assets.go                     # go:embed 声明
├── config/
│   ├── config.example.yaml           # 配置文件示例
│   └── views.example.yaml            # views.yaml 示例
├── cmd/
│   ├── cora/main.go                  # cora 主入口
│   └── smoke/main.go                 # Smoke Runner 入口
├── scenarios/                        # Smoke 测试场景 YAML 文件
├── spec/                             # 架构设计文档
├── Makefile
└── Dockerfile
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

4. （可选）在 `~/.config/cora/views.yaml` 中为常用操作定义自定义 View。

### 后端服务接入要求

- 使用 **OpenAPI 3.0** 规范（不支持 Swagger 2.0）
- 为每个操作声明 `tags`，第一个 tag 即为 CLI 的 `<资源>` 层命令名
- `operationId` 使用已知动词前缀：`list`、`get`、`create`、`update`、`delete`、`patch`
- 通过 `security` 字段按操作声明鉴权需求（缺失或为空表示无需鉴权）

## 退出码

| 退出码 | 含义                   |
|-----|----------------------|
| 0   | 成功                   |
| 1   | API 错误（后端返回 4xx/5xx） |
| 2   | 鉴权错误（未登录或凭证无效）       |
| 3   | 参数校验错误               |
| 4   | Spec 加载失败            |
| 5   | 配置错误（服务未配置或配置文件损坏）   |
| 127 | 未分类错误                |

## Smoke 测试

cora 内置了一套端对端 Smoke 测试框架，用于持续看护各服务子命令的可用性，防止接口不可用或输出异常被遗漏。

### 工作原理

Smoke Runner（`cmd/smoke`）读取 `scenarios/` 目录下的 YAML 场景文件，依次调用真实的 `cora` 二进制，检查退出码、stdout/stderr 内容、响应时间及 JSON 字段等多个维度，最终生成一份 HTML 报告。空文件或纯注释文件会被静默跳过。

### 场景文件格式

```yaml
name: "GitCode · issues list"
service: gitcode
args:
  - issues
  - list
  - --owner
  - openeuler
  - --repo
  - infrastructure
  - --state
  - open
format: table
timeout_ms: 8000
assertions:
  - type: exit_code
    value: 0
  - type: response_time_lt
    value: 5000
  - type: stdout_not_empty
  - type: stderr_not_contains
    value: "ERROR"
  - type: json_has_keys          # 仅 format: json 时有意义
    values: ["title", "state"]
```

**支持的断言类型：**

| 类型 | 说明 |
|------|------|
| `exit_code` | 退出码等于指定值 |
| `stdout_not_empty` | stdout 非空 |
| `stderr_not_contains` | stderr 不包含指定字符串 |
| `response_time_lt` | 响应时间（毫秒）低于指定值 |
| `json_has_keys` | JSON 输出包含所有指定顶层键 |
| `json_key_not_empty` | JSON 指定键值非空 |
| `table_has_columns` | 表格输出包含所有指定列名 |
| `stdout_contains` | stdout 包含指定字符串 |
| `stderr_empty` | stderr 为空 |
| `exit_code_not` | 退出码不等于指定值 |

### 配置

复制示例配置并填写凭证：

```bash
cp config/smoke-config.example.yaml config/smoke-config.yaml
# 编辑 smoke-config.yaml，填入实际 token 和 URL
```

各服务的凭证也可通过环境变量注入，场景文件的 `args` 中可使用 `${VAR}` 占位符：

```bash
export SMOKE_GITCODE_TOKEN=glpat-xxxx
```

### 运行

```bash
# 构建 smoke-runner 并运行全部场景
make smoke

# 只运行包含关键字的场景（按 name 过滤）
make smoke-filter FILTER=gitcode

# 手动运行，指定各参数
./bin/smoke-runner \
  --cora-bin ./bin/cora \
  --config ./config/smoke-config.yaml \
  --scenarios-dir ./scenarios \
  --report-dir ./smoke-report
```

报告默认输出到 `smoke-report/<YYYY-MM-DD>/report.html`，按日期归档。

### CI 集成

GitHub Actions 配置（`.github/workflows/smoke.yml`）每晚 UTC 02:00 自动运行 Smoke 测试，也支持手动触发。HTML 报告作为 Artifact 上传，保留 90 天，名称格式为 `smoke-report-<YYYY-MM-DD>`。

### 目录结构

```
scenarios/
├── gitcode/
│   ├── repos-list.yaml
│   ├── issues-list.yaml
│   └── issues-get.yaml
├── forum/
│   └── posts-list.yaml      # 可注释掉暂时跳过
└── etherpad/
    └── pad-list.yaml        # 可注释掉暂时跳过

internal/smoke/
├── types.go                 # Scenario、Assertion、Result 类型定义
├── loader.go                # YAML 场景加载，空文件自动跳过
├── assertion.go             # 10 种断言逻辑
├── runner.go                # 调用 cora 二进制，注入环境变量
└── report.go                # 控制台输出 + HTML 报告生成
```

## 架构文档

完整架构设计（含框架选型、OpenAPI 驱动命令生成、鉴权策略、Spec 缓存、View 系统等）请参阅 [`spec/`](spec/) 目录。

## 贡献

欢迎贡献代码。提交较大改动前请先开 Issue 说明意图。
