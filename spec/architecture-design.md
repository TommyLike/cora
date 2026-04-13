# Community CLI 架构设计文档

> **版本**：v0.1
> **日期**：2026-04-12
> **状态**：草稿
> **参考**：[googleworkspace/cli 设计模式分析](./reference-cli-design-patterns.md)

---

## 1. 需求概述

### 1.1 功能性需求

| 需求 | 描述 |
|------|------|
| 多服务聚合 | 单一二进制文件访问邮件列表、会议、代码 Issue 等社区服务 |
| 服务动态扩展 | 后端服务通过发布 OpenAPI Spec 驱动 CLI 命令注册，CLI 无需修改代码 |
| 按需鉴权 | 是否需要鉴权由各服务的 OpenAPI Spec Security 定义决定，每个操作独立声明 |
| 统一命令结构 | `community <service> <resource> <verb> [flags]` 四层结构 |
| 多格式输出 | 支持 table / json / yaml / csv |
| 跨服务工作流 | Recipe 层支持跨服务编排（如 Issue → 创建会议） |

### 1.2 非功能性需求

| 需求 | 目标 |
|------|------|
| 启动速度 | 冷启动 < 200ms（Spec 本地缓存，懒加载） |
| 扩展性 | 新增服务零代码改动，仅需后端发布 OpenAPI Spec |
| 安全性 | 凭证加密存储，终端输出防注入，输入严格校验 |
| 可维护性 | 核心框架与服务逻辑解耦，服务适配器独立演进 |
| 脚本友好 | stdout/stderr 分离，退出码语义化，JSON 输出合法可 pipe |

---

## 2. 整体架构

### 2.1 架构分层

```
┌─────────────────────────────────────────────────────────┐
│                     CLI 入口层                           │
│            cobra root command + 全局 flags               │
│        (--format, --output, --dry-run, --profile)        │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│                   服务注册 & 命令生成层                    │
│   ServiceRegistry → 读取/缓存 OpenAPI Spec               │
│   CommandBuilder  → Spec 转 Cobra 命令树（动态）          │
│   HelperRegistry  → 服务特化扩展点（仅跨步骤编排时使用）    │
└──────────┬────────────────┬────────────────┬────────────┘
           │                │                │
┌──────────▼──────┐ ┌───────▼──────┐ ┌──────▼──────────┐
│   Auth 层        │ │  执行层       │ │  Recipe 层       │
│ 分层凭证查找      │ │ HTTP Client  │ │ 跨服务工作流编排  │
│ 多 Provider 支持  │ │ 请求构建/发送 │ │ (独立于命令层)   │
│ Keyring/加密文件  │ │ 响应解析     │ │                 │
└─────────────────┘ └──────────────┘ └─────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│                     输出格式化层                           │
│        Formatter: table / json / yaml / csv              │
│        分页感知 · 智能数据提取 · 终端安全输出               │
└─────────────────────────────────────────────────────────┘
```

### 2.2 命令结构

```
community
├── auth                        # 统一认证入口（内置，非 schema 驱动）
│   ├── login  [--service X]
│   ├── logout [--service X]
│   ├── status
│   └── token  [--service X]
│
├── config                      # 配置管理（内置）
│   ├── init
│   ├── set
│   └── list
│
├── <service>                   # 动态生成，来自 OpenAPI Spec
│   └── <resource>
│       ├── list   [flags]
│       ├── get    [flags]
│       ├── create [flags]
│       └── delete [flags]
│
└── recipe                      # 跨服务工作流（内置编排，按需扩展）
    ├── issue-to-meeting
    ├── meeting-summary
    └── release-announce
```

### 2.3 核心数据流

```
用户执行命令
     │
     ▼
[1] Cobra 解析第一层参数（service 名称）
     │
     ▼
[2] ServiceRegistry 查找服务配置 → SpecLoader 执行三段式加载
     │   ├── 本地缓存存在且未过期（默认 24h）→ 直接读缓存，跳过网络
     │   ├── 缓存过期或不存在 → GET {spec_url} → 写缓存
     │   └── 网络失败且有过期缓存 → 降级使用过期缓存 + stderr 警告
     │
     ▼
[3] CommandBuilder 将 Spec 转为 Cobra 子命令树（注入当前命令）
     │
     ▼
[4] Cobra 重新解析完整参数（带动态子命令）
     │
     ▼
[5] Auth 检查（读取 Spec 的 Security 定义）
     │   ├── 无需鉴权 → 跳过
     │   └── 需要鉴权 → 分层凭证查找 → 注入 Authorization header
     │
     ▼
[6] HTTP Client 执行请求（支持 --dry-run 预览）
     │
     ▼
[7] Formatter 格式化输出（table/json/yaml/csv）
```

---

## 3. 核心组件设计

### 3.1 ServiceRegistry — 服务注册表

服务注册分两种来源，统一通过 `ServiceRegistry` 管理：

**服务条目定义**
```go
// internal/registry/registry.go
type ServiceEntry struct {
    Name    string   // 服务名，如 "mail"
    Aliases []string // 别名，如 ["mailing-list"]
    Loader  SpecLoader
    Helper  Helper   // 可选，仅跨步骤编排时实现
}
```

**SpecLoader — 三段式加载（核心）**

```go
// internal/spec/loader.go

// SpecLoader 封装 Spec 的完整加载策略：优先本地缓存，缓存失效再远端拉取。
type SpecLoader interface {
    Load(ctx context.Context) (*openapi3.T, error)
    Invalidate() error // 强制清除缓存，下次 Load 触发远端拉取
}

// CachedRemoteLoader 是唯一实现，适用于所有配置文件中的服务。
type CachedRemoteLoader struct {
    SpecURL   string        // 完整 URL，如 "https://lists.example.org/openapi.yaml"
    CacheFile string        // 本地缓存路径，如 "~/.config/community-cli/cache/mail.json"
    TTL       time.Duration // 缓存有效期，默认 24h
}

func (l *CachedRemoteLoader) Load(ctx context.Context) (*openapi3.T, error) {
    // 1. 读取本地缓存文件
    if cached, err := l.readCache(); err == nil {
        if time.Since(cached.FetchedAt) < l.TTL {
            // 缓存未过期：直接返回，不发起任何网络请求
            return cached.Spec, nil
        }
    }

    // 2. 缓存过期或不存在：从远端拉取
    spec, err := l.fetchRemote(ctx)
    if err != nil {
        // 3. 网络失败降级：使用过期缓存（如果有），并在 stderr 警告
        if stale, cacheErr := l.readCache(); cacheErr == nil {
            fmt.Fprintf(os.Stderr, "[warn] failed to refresh spec for %s: %v (using cached version from %s)\n",
                l.SpecURL, err, stale.FetchedAt.Format(time.RFC3339))
            return stale.Spec, nil
        }
        return nil, fmt.Errorf("fetch spec %s: %w", l.SpecURL, err)
    }

    // 4. 写入缓存（原子写入，防止并发污染）
    _ = l.writeCache(spec)
    return spec, nil
}

// 缓存文件结构
type specCache struct {
    FetchedAt time.Time   `json:"fetched_at"`
    SpecURL   string      `json:"spec_url"`
    Spec      *openapi3.T `json:"spec"`
}
```

**服务发现流程（对齐 Google CLI 模式）**
```
用户执行 community <service> <resource> <verb>
     │
     ▼
[1] 从配置文件读取该 service 的 spec_url
     │
     ▼
[2] CachedRemoteLoader.Load()
     ├── ~/.config/community-cli/cache/<service>.json 存在且 fetched_at 在 24h 内
     │       └── ✓ 直接返回缓存 Spec（无网络请求）
     │
     ├── 缓存不存在 / 已过期
     │       └── GET {spec_url}
     │               ├── 成功 → 写缓存 → 返回 Spec
     │               └── 失败且有旧缓存 → 返回旧缓存 + stderr 警告
     │
     └── 失败且无任何缓存 → 退出码 4，提示检查网络和 spec_url 配置
     │
     ▼
[3] CommandBuilder 将 Spec 转为 Cobra 子命令树
```

**服务配置文件（用户可扩展）**
```yaml
# ~/.config/community-cli/config.yaml
services:
  mail:
    spec_url: https://lists.example.org/openapi.yaml
  meeting:
    spec_url: https://calendar.example.org/openapi.yaml
  issue:
    spec_url: https://github-proxy.example.org/openapi.yaml
  # 用户自定义服务（任何暴露 OpenAPI Spec 的服务均可接入）
  custom-wiki:
    spec_url: https://wiki.myorg.org/openapi.yaml

# 全局 Spec 缓存配置（可选，有默认值）
spec_cache:
  ttl: 24h            # 缓存有效期，默认 24 小时
  dir: ~/.config/community-cli/cache  # 缓存目录
```

**缓存文件布局**
```
~/.config/community-cli/
└── cache/
    ├── mail.json        # {"fetched_at": "2026-04-12T10:00:00Z", "spec_url": "...", "spec": {...}}
    ├── meeting.json
    └── issue.json
```

---

### 3.2 CommandBuilder — OpenAPI 转 Cobra 命令

将 OpenAPI 3.0 Spec 动态生成 Cobra 命令树，**无需修改核心代码**。

**映射规则**

| OpenAPI 概念 | CLI 映射 |
|-------------|---------|
| `tags[0]` | resource 名称（第三层命令） |
| `operationId` → 去掉 tag 前缀后的动词 | verb 名称（第四层命令） |
| `parameters[].in == "query"` | `--flag` |
| `parameters[].in == "path"` | positional arg 或 `--flag` |
| `requestBody` | `--data` (JSON) 或展开为具体 flags |
| `security` 非空 | 标记该命令需要鉴权 |
| `description` | `--help` 文本 |
| `x-cli-examples` | Help 中的 Examples 段落（扩展字段） |

**参数生成策略**：
- 常用参数（≤5 个）展开为具体 `--flag`
- 其余参数通过 `--params '{"key":"val"}'` JSON 透传（防止 flag 爆炸）
- 必填参数自动调用 `cmd.MarkFlagRequired()`

**OpenAPI 扩展字段（x-cli-* ）**

后端服务可通过扩展字段增强 CLI 体验，无需修改 CLI 代码：

```yaml
# 后端 openapi.yaml 示例
paths:
  /v1/threads:
    get:
      operationId: mail_threads_list
      tags: [threads]
      x-cli-examples:
        - "community mail threads list --filter 'subject:release'"
        - "community mail threads list --limit 100 --format json"
      x-cli-flags:         # 明确指定展开为 flag 的参数（覆盖自动推断）
        - filter
        - limit
        - after
      parameters:
        - name: filter
          in: query
          schema: { type: string }
          description: "Filter expression"
        - name: limit
          in: query
          schema: { type: integer, default: 20 }
      security:
        - BearerAuth: []   # 有此字段则该命令需要鉴权
```

---

### 3.3 Auth 层 — 按命令声明式鉴权

**核心原则**：鉴权需求由 OpenAPI Spec 的 `security` 字段声明，CLI 不硬编码哪些命令需要鉴权。

**凭证查找优先级**（与 Google CLI 对齐）

```
1. 环境变量  COMMUNITY_<SERVICE>_TOKEN
              COMMUNITY_<SERVICE>_API_KEY
   ↓
2. 环境变量指定文件  COMMUNITY_CREDENTIALS_FILE
   ↓
3. OS Keyring（macOS Keychain / Windows Credential Store）
   ↓
4. 加密凭证文件  ~/.config/community-cli/credentials.enc（AES-256-GCM）
   ↓
5. 明文凭证文件  ~/.config/community-cli/credentials.json（降级）
```

**多种 Auth Provider**

```go
// internal/auth/provider.go
type Provider interface {
    // 判断是否能为该服务提供凭证
    Supports(service string, scheme openapi3.SecurityScheme) bool
    // 将凭证注入请求头
    Inject(req *http.Request, creds Credentials) error
}

// 实现列表
var providers = []Provider{
    &BearerTokenProvider{},   // Authorization: Bearer <token>
    &APIKeyProvider{},        // X-API-Key: <key> 或 ?api_key=<key>
    &BasicAuthProvider{},     // Authorization: Basic <base64>
    &OAuth2Provider{},        // OAuth2 Device Flow / PKCE
}
```

**命令级鉴权决策流**

```
执行命令前：
  读取该 operation 的 security 字段
    ├── security: []  或 无 security 字段 → 跳过鉴权
    └── security: [{BearerAuth: []}]
            │
            ▼
        查找凭证（按优先级）
            ├── 找到 → 注入请求头
            └── 找不到 → 提示 "community auth login --service <name>"
                        并以退出码 2 退出
```

---

### 3.4 HTTP 执行层

```go
// internal/executor/executor.go
type Executor struct {
    client    *http.Client
    formatter output.Formatter
    dryRun    bool
}

func (e *Executor) Execute(ctx context.Context, req *ExecuteRequest) error {
    // 1. 构建 HTTP 请求（URL、Headers、Body）
    // 2. dry-run：打印请求详情，不发送
    // 3. 发送请求
    // 4. 错误处理（状态码映射到退出码）
    // 5. 分页处理（检测 nextPageToken / next_cursor）
    // 6. 输出格式化
}
```

**分页自动处理**

```go
// 检测 OpenAPI 响应 schema 中的分页字段
// 支持两种风格（与团队 API 规范对齐）：
//   偏移量分页：pagination.has_next + ?page=N
//   游标分页：next_cursor + ?cursor=X
```

---

### 3.5 输出格式化层

```go
// internal/output/formatter.go
type Formatter interface {
    Format(w io.Writer, data interface{}) error
    // 分页场景：首页写 header，后续页追加数据（不重复 header）
    FormatPage(w io.Writer, data interface{}, isFirst bool) error
}

type Format string
const (
    FormatTable Format = "table"
    FormatJSON  Format = "json"
    FormatYAML  Format = "yaml"
    FormatCSV   Format = "csv"
)
```

**智能数据提取**：自动跳过 `nextPageToken`、`kind`、`_`前缀字段，识别实际列表数据。

**终端安全**：所有输出过滤 ASCII 控制字符（0x00-0x1F, 0x7F），遵守 `NO_COLOR` 环境变量。

---

### 3.6 Helper — 服务特化扩展点（严格受限）

只在以下场景实现 Helper，**单一 API 操作禁止使用 Helper**：

```go
// internal/helper/helper.go
type Helper interface {
    // 注入额外子命令（以 + 前缀区分，如 "+send"）
    InjectCommands(parent *cobra.Command, spec *openapi3.T) *cobra.Command
    // 拦截命令执行（可选，不实现则走通用执行路径）
    Handle(ctx context.Context, cmd *cobra.Command, args []string) (bool, error)
}
```

**判断标准**：用户能否通过 `community <service> <resource> <verb> --params '{...}'` 完成任务？能 → 不需要 Helper。

**初期规划的 Helper 场景**：

| Helper | 场景 | 说明 |
|--------|------|------|
| `mail.Helper` | `+send`：构建 MIME 邮件 + 附件 | 原始参数无法表达多 part |
| `recipe.Helper` | 跨服务编排 | Issue → Meeting、Tag → 邮件公告 |

---

## 4. 目录结构

```
community-cli/
├── cmd/
│   └── community/
│       └── main.go                  # 入口，初始化 ServiceRegistry，构建 root command
│
├── internal/
│   ├── registry/
│   │   ├── registry.go              # ServiceRegistry：服务注册与查找
│   │   ├── entry.go                 # ServiceEntry 定义
│   │   └── builtin.go               # 内置服务注册（init 函数注册）
│   │
│   ├── spec/
│   │   ├── loader.go                # SpecLoader 接口 + CachedRemoteLoader 实现（三段式加载）
│   │   ├── cache.go                 # 缓存读写（原子写入、TTL 判断、specCache 结构体）
│   │   └── parser.go                # OpenAPI 3.0 解析与校验
│   │
│   ├── builder/
│   │   ├── command.go               # CommandBuilder：Spec → Cobra 命令
│   │   ├── flags.go                 # 参数 → Flag 映射逻辑
│   │   └── help.go                  # Help 文本生成（含 x-cli-examples）
│   │
│   ├── auth/
│   │   ├── provider.go              # Provider 接口
│   │   ├── bearer.go                # BearerToken Provider
│   │   ├── apikey.go                # APIKey Provider
│   │   ├── oauth2.go                # OAuth2 Provider（Device Flow）
│   │   ├── store.go                 # 凭证存储（Keyring + 加密文件）
│   │   └── resolver.go              # 分层凭证查找
│   │
│   ├── executor/
│   │   ├── executor.go              # HTTP 请求执行
│   │   ├── paginator.go             # 分页处理
│   │   └── sanitizer.go             # 输入校验 + 终端安全输出
│   │
│   ├── output/
│   │   ├── formatter.go             # Formatter 接口
│   │   ├── table.go                 # Table 格式
│   │   ├── json.go                  # JSON 格式
│   │   ├── yaml.go                  # YAML 格式
│   │   └── csv.go                   # CSV 格式
│   │
│   ├── helper/
│   │   ├── helper.go                # Helper 接口
│   │   ├── mail/
│   │   │   └── send.go              # 邮件发送 Helper（MIME 构建）
│   │   └── recipe/
│   │       ├── issue_to_meeting.go
│   │       └── release_announce.go
│   │
│   └── config/
│       ├── config.go                # 配置结构体
│       ├── loader.go                # 分层加载（系统→用户→项目→环境变量→flags）
│       └── profile.go               # 多 Profile 管理
│
├── pkg/
│   └── errs/
│       └── errors.go                # 公共错误类型 + 退出码定义
│
├── spec/                            # 设计文档
├── docs/
│   └── adr/                         # 架构决策记录
│       ├── 0001-cli-framework.md
│       ├── 0002-openapi-driven-commands.md
│       ├── 0003-auth-strategy.md
│       └── 0004-spec-caching.md
│
└── Makefile
```

---

## 5. 架构决策记录（ADR）

### ADR-0001：CLI 框架选型 — Cobra

**状态**：已接受

**背景**：需要支持 OpenAPI 驱动的动态命令生成。

**决策**：使用 `github.com/spf13/cobra`。

**理由**：
- Go 社区事实标准（kubectl、helm、gh 均采用）
- 纯 builder API（运行时构建命令树），天然支持动态命令注册，无需两阶段解析
- `PersistentPreRunE` 钩子支持懒加载 Spec 后注入子命令
- 内置 shell completion 框架（zsh/bash/fish/powershell）

**代价**：
- Shell completion 与动态命令需要额外处理（completion 时全量加载所有 Spec）

---

### ADR-0002：OpenAPI Spec 驱动命令生成（对齐 Google CLI 模式）

**状态**：已接受

**背景**：需要在不修改 CLI 代码的前提下支持后端服务扩展新 API。

**决策**：
1. 后端服务在 `GET /openapi.yaml` 发布 OpenAPI 3.0 Spec
2. CLI 启动时，仅解析第一层参数（service 名）
3. 从服务端拉取（或读取缓存）该服务的 Spec
4. 动态构建完整 Cobra 命令树，重新解析全量参数

**理由**：
- 服务 API 迭代后 CLI 自动更新，无需发新版本
- OpenAPI 是开放标准，后端团队无额外学习成本
- `x-cli-*` 扩展字段允许后端针对 CLI 场景优化体验

**约束**：
- 后端服务的 OpenAPI Spec 端点必须无需鉴权（公开可访问）
- Spec 格式必须为 OpenAPI 3.0（不支持 Swagger 2.0）

---

### ADR-0003：鉴权声明在 OpenAPI Spec，CLI 不硬编码

**状态**：已接受

**决策**：每个 operation 是否需要鉴权，完全由 OpenAPI Spec 的 `security` 字段声明：
- `security: []` 或不含 `security` → 无需鉴权
- `security: [{BearerAuth: []}]` → 需要 Bearer Token

**理由**：
- 鉴权需求随业务变化，hardcode 在 CLI 会造成版本耦合
- 符合 OpenAPI 规范语义，后端开发者已熟悉
- 支持混合场景：同一服务部分接口公开，部分需要鉴权

---

### ADR-0004：Spec 本地缓存策略

**状态**：已接受

**决策**：所有 OpenAPI Spec 通过 `CachedRemoteLoader` 统一加载，缓存路径为 `~/.config/community-cli/cache/<service>.json`，默认 TTL **24 小时**。

**三段式加载行为（严格优先级）**：

| 优先级 | 条件 | 行为 |
|--------|------|------|
| 1（最高） | 缓存存在 且 `fetched_at` 在 TTL 内 | **直接读本地文件，不发起任何网络请求** |
| 2 | 缓存不存在 或 已过期 | GET `{spec_url}` → 成功则写缓存并返回 |
| 3 | 网络请求失败 且 存在过期缓存 | 返回过期缓存，stderr 输出警告（含过期时间） |
| 4（兜底） | 网络失败 且 无任何缓存 | 退出码 4，提示检查网络和 `spec_url` 配置 |

**强制刷新**：`--refresh-spec` flag 跳过优先级 1，直接执行优先级 2 流程。

**理由**：
- 冷启动要求 < 200ms，**缓存有效时零网络开销**是核心设计目标
- 社区服务 API 变更频率低（日级），24h TTL 远比 1h 更符合实际使用节奏
- 网络降级策略确保离线或弱网环境下 CLI 仍可用
- 缓存 TTL 可在配置文件中覆盖，满足不同场景需求（如开发阶段设为 `5m`）

---

### ADR-0005：错误退出码规范

**状态**：已接受

| 退出码 | 含义 |
|--------|------|
| 0 | 成功 |
| 1 | API 错误（服务端 4xx/5xx） |
| 2 | 认证错误（未登录、Token 过期） |
| 3 | 参数校验错误（用户输入有误） |
| 4 | Spec 获取失败（无法拉取 OpenAPI Spec） |
| 5 | 配置错误（服务未配置、配置文件损坏） |
| 127 | 其他未分类错误 |

---

## 6. 后端服务接入规范

后端服务接入 community-cli 需满足以下要求：

### 6.1 必须项

```yaml
# 1. 在根路径发布 OpenAPI 3.0 Spec（无需鉴权）
GET /openapi.yaml  →  Content-Type: application/yaml

# 2. Info 对象包含服务元信息
info:
  title: "Mail List Service"
  version: "v1"
  x-cli-name: "mail"            # CLI 命令中使用的服务名
  x-cli-aliases: ["mailing-list"] # 可选别名

# 3. 使用 tags 组织资源（对应 CLI 第三层命令）
tags:
  - name: threads
    description: "Mailing list threads"

# 4. Operation ID 遵循命名规范
# 格式：{tag}_{verb}  或  {tag}_{sub-resource}_{verb}
operationId: threads_list      # → community mail threads list
operationId: threads_get       # → community mail threads get
operationId: threads_reply     # → community mail threads reply

# 5. 分页响应遵循团队 API 规范（偏移量或游标二选一）
```

### 6.2 推荐项（增强 CLI 体验）

```yaml
# x-cli-examples：在 --help 中展示示例
x-cli-examples:
  - "community mail threads list --filter 'subject:release'"

# x-cli-flags：明确指定展开为独立 flag 的参数（其余走 --params）
x-cli-flags: [filter, limit, after]

# x-cli-flags-required：标记必填 flag（补充 OpenAPI required 字段）
x-cli-flags-required: [thread-id]
```

### 6.3 服务注册（初期：配置文件驱动）

```yaml
# ~/.config/community-cli/config.yaml
services:
  mail:
    url: https://lists.example.org
  meeting:
    url: https://calendar.example.org
  issue:
    url: https://github-proxy.example.org
```

---

## 7. 关键接口定义

```go
// =====================
// 服务注册
// =====================
type ServiceRegistry interface {
    Register(entry ServiceEntry) error
    Lookup(name string) (*ServiceEntry, error)
    All() []ServiceEntry
}

// =====================
// Spec 加载（三段式：本地缓存优先 → 远端拉取 → 降级旧缓存）
// =====================
type SpecLoader interface {
    Load(ctx context.Context) (*openapi3.T, error)
    Invalidate() error // 清除缓存，下次 Load 强制走远端
}

// =====================
// 命令构建
// =====================
type CommandBuilder interface {
    Build(service string, spec *openapi3.T) (*cobra.Command, error)
}

// =====================
// 服务特化扩展
// =====================
type Helper interface {
    InjectCommands(parent *cobra.Command, spec *openapi3.T) *cobra.Command
    Handle(ctx context.Context, cmd *cobra.Command, args []string) (bool, error)
}

// =====================
// 认证 Provider
// =====================
type AuthProvider interface {
    Supports(service string, scheme *openapi3.SecurityScheme) bool
    Inject(req *http.Request, creds *Credentials) error
}

// =====================
// 输出格式化
// =====================
type Formatter interface {
    Format(w io.Writer, data any) error
    FormatPage(w io.Writer, data any, isFirst bool) error
}

// =====================
// 错误类型（对应退出码）
// =====================
type CLIError struct {
    Code     ExitCode
    Message  string
    Hint     string    // 操作建议，输出到 stderr
    Cause    error
}
```

---

## 8. 典型交互示例

```bash
# 初始化配置
community config init
community auth login --service mail

# 查询邮件列表
community mail threads list --filter "subject:release"
community mail threads list --limit 100 --format json | jq '.[].subject'

# 查询 Issue（无需鉴权的公开接口）
community issue issues list --repo cncf/xxx --state open

# 查询 Issue（需要鉴权的接口）
community issue issues create --repo cncf/xxx --title "Bug" --dry-run

# 跨服务 Recipe
community recipe release-announce --tag v1.2.0 --repo cncf/xxx --list dev@example.org

# 强制刷新 Spec 缓存
community mail threads list --refresh-spec

# 调试：查看实际发出的 HTTP 请求
community mail threads list --dry-run --format json
```

---

## 9. 风险与缓解

| 风险 | 影响 | 缓解方案 |
|------|------|---------|
| 后端 Spec 不规范（operationId 缺失、tag 混乱） | 命令生成错误或无法生成 | 提供 Spec 校验工具 `community spec validate --service X`；接入 CI 检查 |
| 网络不可用导致 Spec 拉取失败 | 命令无法执行 | 过期缓存降级使用，内置服务可 embed 默认 Spec 作为兜底 |
| Spec 更新后 CLI 命令结构变化 | 用户脚本中断 | 服务端遵守 API 兼容性规范（ADR-0003 约束）；提供 `--spec-version` 锁定 |
| Shell completion 性能（需全量加载所有 Spec） | Completion 响应慢 | Completion 场景并发拉取所有 Spec；缓存预热命令 `community spec prefetch` |
| 凭证文件泄露 | 安全风险 | 默认 AES-256-GCM 加密；0600 权限；定期轮转提示 |

---

## 10. 里程碑规划

| 阶段 | 目标 | 交付物 |
|------|------|--------|
| **M1 — 核心框架** | 可运行的动态命令框架 | ServiceRegistry + CommandBuilder + Executor + Formatter |
| **M2 — 首批服务** | 接入 mail/issue/meeting 三个服务 | 三个服务的 OpenAPI Spec + CLI 集成 + Auth 层 |
| **M3 — 体验完善** | Shell completion + 多 Profile + Recipe 层 | Completion 脚本；`--dry-run`；2~3 个 Recipe |
| **M4 — 生产就绪** | 发布 v1.0 | 文档；安装脚本；CI/CD 流水线 |
