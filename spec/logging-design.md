# Cora 日志系统设计文档

> **版本**：v1.0
> **日期**：2026-04-17
> **状态**：已接受
> **关联 ADR**：ADR-0006

---

## 1. 背景与目标

### 1.1 问题

当前日志输出存在以下问题：

- 输出分散在各包，使用 `fmt.Fprintln(os.Stderr, ...)` 硬编码，无统一控制
- 无日志级别，用户无法按需开启调试信息
- 无敏感信息脱敏，API Key / Token 可能泄露到终端或日志文件
- 出现问题时缺乏足够上下文，排查困难

### 1.2 目标

1. 引入统一的 `internal/log` 包，提供分级日志能力
2. 通过 `--verbose` 全局 Flag 按需开启详细输出
3. 在关键节点埋点，覆盖配置加载、Spec 缓存、认证注入、HTTP 请求/响应全链路
4. 自动脱敏 URL 中的敏感参数和请求头，防止凭证泄露

---

## 2. 日志级别设计

| 级别    | 前缀      | 默认显示 | 用途 |
|---------|-----------|---------|------|
| `ERROR` | `[ERROR]` | 始终     | 不可恢复的失败，程序终止或请求中断 |
| `WARN`  | `[WARN]`  | 始终     | 降级行为，如使用过期缓存、服务未找到 |
| `INFO`  | `[INFO]`  | `--verbose` 时 | 正常流程的关键状态，如 Spec 加载完成 |
| `DEBUG` | `[DEBUG]` | `--verbose` 时 | 细粒度诊断，如请求 URL、响应体、认证提供方 |

**设计原则：**

- CLI 工具默认安静（quiet by default），只输出用户数据和必要警告
- `--verbose` 同时开启 INFO 和 DEBUG，不再细分
- ERROR/WARN 输出到 `stderr`；INFO/DEBUG 也输出到 `stderr`（避免污染 stdout 数据管道）
- 无时间戳（CLI 工具无需，时间戳由 shell history 提供）

---

## 3. 包结构

```
internal/log/
  log.go       # Logger 核心：Init()、全局函数、格式化输出
  mask.go      # 敏感信息脱敏：MaskURL()、MaskHeader()
  mask_test.go # 脱敏单元测试
```

### 3.1 log.go API

```go
package log

// Init 在 main() 最早处调用，根据 --verbose flag 设置全局日志级别。
// verbose=true  → 开启 INFO + DEBUG
// verbose=false → 仅显示 WARN（ERROR 由 errs 包处理）
func Init(verbose bool)

// 四个级别函数，均写入 stderr，支持 fmt.Sprintf 风格格式化。
func Error(format string, args ...any)  // 始终显示
func Warn(format string, args ...any)   // 始终显示
func Info(format string, args ...any)   // verbose 时显示
func Debug(format string, args ...any)  // verbose 时显示
```

输出示例：

```
[WARN]  using stale cache for "gitcode" (fetched 26h ago); re-fetch with --refresh-spec
[INFO]  spec loaded for "gitcode" (184 paths, cache age: 3m)
[INFO]  config loaded from /Users/user/.config/cora/config.yaml
[DEBUG] .env loaded from /project/.env (3 vars applied)
[DEBUG] → GET https://api.gitcode.com/api/v5/repos/openeuler/community/issues/1367?access_token=***
[DEBUG] ← 200 OK (1247 bytes, 234ms)
[DEBUG] response body: {"id":3851910,"title":"Base-Service Sig添加committer",...}
[DEBUG] auth: injecting gitcode access_token for service "gitcode"
```

### 3.2 mask.go API

```go
package log

// MaskURL 将 URL 中的敏感 query 参数值替换为 "***"。
// 被脱敏的参数名列表：access_token, apikey, api_key, token, secret, password, key
func MaskURL(rawURL string) string

// MaskHeader 返回脱敏后的 Header 副本，不修改原始 Header。
// 被脱敏的 Header 名列表：Api-Key, Api-Username, Authorization
func MaskHeader(h http.Header) http.Header
```

---

## 4. 敏感信息脱敏策略

### 4.1 脱敏范围

| 数据类型 | 处理方式 |
|---------|---------|
| URL query 参数（access_token 等） | 值替换为 `***`，参数名保留 |
| 请求头（Api-Key、Authorization 等） | 值替换为 `***`，Header 名保留 |
| 响应 body | **原样记录**（verbose 模式下是关键调试信息），但截断超长内容（默认 2KB） |
| 配置文件中的 key 值 | **永不记录**（只记录配置文件路径，不记录内容） |
| .env 文件中的变量 | **只记录变量名**，不记录值 |

### 4.2 响应 Body 截断规则

```
body 长度 ≤ 2048 字节 → 完整记录
body 长度 > 2048 字节 → 记录前 2048 字节，追加 "... [truncated, total: Xbytes]"
```

响应 body 是诊断 API 错误的关键信息，verbose 模式下必须显示。截断防止超大响应（如文件下载）淹没终端。

### 4.3 脱敏字段清单（固定，不动态扩展）

**Query 参数：**
```
access_token, apikey, api_key, token, secret, password, key
```

**请求头：**
```
Api-Key, Api-Username, Authorization
```

---

## 5. 埋点位置

### 5.1 全链路埋点图

```
cmd/cora/main.go → run()
  [INFO]  config loaded from <path>

cmd/cora/main.go → loadDotEnv (via config)
  [DEBUG] .env loaded from <path> (<N> vars applied)

cmd/cora/main.go → injectServiceCommands()
  [DEBUG] looking up service "<name>"
  [INFO]  loading spec for "<name>"

internal/spec/loader.go → Load()
  [INFO]  cache hit for "<name>" (age: <duration>, TTL: <ttl>)
  [INFO]  fetching spec for "<name>" from <url>
  [INFO]  spec cached to <cache_file>
  [WARN]  fetch failed, using stale cache (age: <duration>): <error>

internal/auth/resolver.go → InjectAuth()
  [DEBUG] auth: injecting discourse headers for service "<name>"
  [DEBUG] auth: injecting etherpad apikey for service "<name>"
  [DEBUG] auth: injecting gitcode access_token for service "<name>"

internal/executor/executor.go → Execute()
  [DEBUG] → <METHOD> <masked_url>  [body: <N> bytes]
  [DEBUG] ← <status_code> <status_text> (<N> bytes, <duration>ms)
  [DEBUG] response body: <body_or_truncated>
```

### 5.2 埋点详细说明

#### main.go

```go
// config 加载后
log.Info("config loaded from %s", path)

// .env 加载后（在 config.loadDotEnv 内）
log.Debug(".env loaded from %s (%d vars applied)", envFile, count)

// service 查找
log.Debug("looking up service %q", svcName)

// spec 加载前
log.Info("loading spec for %q", svcName)
```

#### spec/loader.go

```go
// Tier 1：缓存命中
log.Info("cache hit for %q (age: %s, TTL: %s)", name, age.Round(time.Second), l.TTL)

// Tier 2：拉取
log.Info("fetching spec for %q from %s", name, l.SpecURL)
// 写缓存成功后
log.Info("spec cached to %s", l.CacheFile)

// Tier 3：降级
log.Warn("fetch failed for %q, using stale cache (age: %s): %v", name, age, fetchErr)
```

#### auth/resolver.go

```go
// 只记录 provider 类型，不记录 key 值
if d := svc.Auth.Discourse; d != nil {
    log.Debug("auth: injecting discourse headers for service %q", svcName)
}
if e := svc.Auth.Etherpad; e != nil && e.APIKey != "" {
    log.Debug("auth: injecting etherpad apikey for service %q", svcName)
}
if g := svc.Auth.Gitcode; g != nil && g.AccessToken != "" {
    log.Debug("auth: injecting gitcode access_token for service %q", svcName)
}
```

#### executor/executor.go

```go
// 请求前（auth 注入后，dry-run 检查前）
bodySize := 0
if len(req.Body) > 0 { bodySize = len(bodyJSON) }
log.Debug("→ %s %s  [body: %d bytes]", req.Method, log.MaskURL(httpReq.URL.String()), bodySize)

// 响应后
log.Debug("← %s (%d bytes, %dms)", resp.Status, len(respBytes), elapsed.Milliseconds())
log.Debug("response body: %s", truncateBody(respBytes, 2048))
```

---

## 6. 全局 Flag 接入

在 `cmd/cora/main.go` 的 root command 上添加：

```go
root.PersistentFlags().Bool("verbose", false, "enable verbose output for debugging")
```

在 `run()` 中最早读取（早于任何其他操作，包括 config 加载）：

```go
func run() error {
    // 预扫描 os.Args 获取 --verbose，因为此时 cobra 还未解析
    verbose := containsFlag(os.Args, "--verbose")
    log.Init(verbose)

    cfg, err := config.Load()
    ...
}
```

> **说明**：必须预扫描 `os.Args` 而非通过 cobra 读取，原因是日志需要在 cobra 解析之前就初始化（config 加载、spec 加载均在 cobra 解析前发生）。

---

## 7. 依赖关系约束

```
internal/log  ←  internal/spec
internal/log  ←  internal/executor
internal/log  ←  internal/auth
internal/log  ←  internal/config
internal/log  ←  cmd/cora

internal/log  不可引用任何其他 internal/* 包（叶子依赖）
```

`internal/log` 是项目的基础依赖，只允许引用标准库（`fmt`, `io`, `net/url`, `net/http`, `os`, `strings`, `sync`）。

---

## 8. 测试要求

`internal/log/mask_test.go` 必须覆盖：

| 测试场景 | 期望结果 |
|---------|---------|
| URL 含 `access_token` | 值替换为 `***`，其他参数不变 |
| URL 含多个敏感参数 | 全部替换 |
| URL 不含敏感参数 | 原样返回 |
| URL 格式非法 | 原样返回，不 panic |
| Header 含 `Api-Key` | 值替换为 `***` |
| Header 不含敏感字段 | 原样返回 |
| 响应 body ≤ 2048 字节 | 完整返回 |
| 响应 body > 2048 字节 | 截断并附加提示 |

---

## 9. 变更文件清单

| 文件 | 操作 |
|------|------|
| `internal/log/log.go` | 新建 |
| `internal/log/mask.go` | 新建 |
| `internal/log/mask_test.go` | 新建 |
| `cmd/cora/main.go` | 添加 `--verbose` flag；调用 `log.Init()`；替换 `fmt.Fprintln(stderr)` |
| `internal/spec/loader.go` | 替换 `fmt.Fprintf(stderr)` → `log.Warn/Info` |
| `internal/executor/executor.go` | 添加请求/响应 DEBUG 埋点 |
| `internal/auth/resolver.go` | 添加 auth provider DEBUG 埋点 |
| `internal/config/config.go` | 添加配置文件路径 INFO 埋点 |
