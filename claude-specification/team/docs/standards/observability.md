# 日志与可观测性规范

> 本文件定义团队在日志、指标（Metrics）、链路追踪（Tracing）方面的统一约定。
> Claude 在编写涉及日志输出、错误处理、监控集成的代码时应遵循本规范。

---

## 可观测性三支柱

| 支柱 | 用途 | 工具 |
| --- | --- | --- |
| 日志（Logging） | 记录离散事件，排查问题细节 | [团队填写] |
| 指标（Metrics） | 量化系统行为，触发告警 | [团队填写] |
| 链路追踪（Tracing） | 跟踪请求在服务间的流转 | [团队填写] |

---

## 日志规范

### 日志级别

| 级别 | 使用场景 | 示例 |
| --- | --- | --- |
| ERROR | 需要人工介入的错误，影响业务功能 | 数据库连接失败、支付回调处理失败 |
| WARN | 异常但可自动恢复，需关注趋势 | 重试成功、缓存击穿降级到 DB |
| INFO | 关键业务动作，正常运行轨迹 | 用户登录、订单创建、服务启动 |
| DEBUG | 开发调试信息，生产环境默认关闭 | 请求参数详情、SQL 查询、缓存命中 |

**规则：**

- 生产环境默认日志级别：`INFO`
- ERROR 不能用于预期内的业务异常（如"用户不存在"应为 WARN 或 INFO）
- 每条 ERROR 日志必须有对应的告警或处理方式
- 不在循环中打 INFO 日志（避免日志洪水）

### 结构化日志格式

所有日志必须使用 **JSON 结构化格式**输出：

```json
{
  "timestamp": "2024-03-15T10:30:00.123Z",
  "level": "INFO",
  "service": "user-service",
  "trace_id": "abc123def456",
  "span_id": "span789",
  "caller": "handler/user.go:42",
  "message": "user login success",
  "user_id": "u-123",
  "latency_ms": 45,
  "http_method": "POST",
  "http_path": "/v1/auth/login",
  "http_status": 200
}
```

### 必备字段

| 字段 | 说明 | 必须 |
| --- | --- | --- |
| `timestamp` | ISO 8601 格式，UTC 时区 | 是 |
| `level` | 日志级别 | 是 |
| `service` | 服务名称 | 是 |
| `trace_id` | 链路追踪 ID | 是 |
| `message` | 日志消息（人类可读） | 是 |
| `caller` | 代码位置（文件:行号） | 是 |
| `error` | 错误详情（仅 ERROR/WARN 级别） | 条件 |

### 上下文字段

根据场景附加业务上下文字段：

```
# HTTP 请求
http_method, http_path, http_status, latency_ms, client_ip

# 数据库操作
db_operation, db_table, db_latency_ms, rows_affected

# 消息队列
mq_topic, mq_partition, mq_offset, mq_consumer_group

# 用户上下文
user_id, tenant_id, request_id
```

### 日志禁忌

1. **禁止记录敏感信息**：密码、Token、信用卡号、身份证号
2. **禁止使用 fmt.Println / console.log 输出日志**——必须用日志框架
3. **禁止日志消息中拼接变量**——使用结构化字段

```
❌ log.Info("user " + userId + " login from " + ip)
✅ log.Info("user login", "user_id", userId, "client_ip", ip)
```

---

## 指标（Metrics）规范

### 命名规范

```
<service>_<domain>_<metric>_<unit>

示例：
user_service_http_request_duration_seconds
order_service_payment_total_count
user_service_db_connection_pool_size
```

**规则：**

- 使用 snake_case
- 单位后缀：`_seconds`、`_bytes`、`_count`、`_total`、`_ratio`
- 不使用缩写（`req` → `request`）

### 必备指标（RED 方法）

每个服务必须暴露以下指标：

| 指标 | 类型 | 说明 |
| --- | --- | --- |
| `<service>_http_request_total` | Counter | 请求总数（按 method、path、status 分组） |
| `<service>_http_request_duration_seconds` | Histogram | 请求延迟分布 |
| `<service>_http_request_errors_total` | Counter | 错误请求数（HTTP 5xx） |

### 业务指标

> [团队填写] 各服务需暴露的核心业务指标，例如：
>
> - 订单创建数、支付成功率、用户注册数
> - 缓存命中率、消息队列积压量

---

## 链路追踪规范

### 集成要求

- 使用 [团队填写] （推荐 OpenTelemetry）作为 Tracing SDK
- 所有 HTTP/gRPC 入口自动创建 Span
- 跨服务调用自动传播 Trace Context（W3C TraceContext 标准）

### Span 命名

```
# HTTP
HTTP <method> <path>
示例：HTTP GET /v1/users

# gRPC
<package>.<Service>/<Method>
示例：user.v1.UserService/GetUser

# 数据库
DB <operation> <table>
示例：DB SELECT users

# 消息队列
MQ <operation> <topic>
示例：MQ PUBLISH order.created
```

### Span 属性

每个 Span 应携带必要属性：

```
# HTTP Span
http.method, http.url, http.status_code, http.request_content_length

# DB Span
db.system, db.statement (脱敏), db.operation, db.name

# 用户上下文
user.id, tenant.id
```

### 采样策略

> [团队填写] 链路追踪采样率：
>
> - 开发/测试环境：100%
> - 生产环境：[填写]%（建议 1%-10%，根据流量调整）
> - ERROR 请求：100%（错误请求始终采集）

---

## 告警规范

### 告警分级

| 级别 | 定义 | 响应要求 |
| --- | --- | --- |
| P0 | 核心业务完全不可用 | [团队填写] 分钟内响应 |
| P1 | 核心业务部分受损或严重降级 | [团队填写] 分钟内响应 |
| P2 | 非核心功能异常 | 工作时间内处理 |
| P3 | 预警，趋势异常 | 下个迭代处理 |

### 告警规则设计原则

1. **有告警必须有 Runbook**——告警触发后怎么排查、怎么处理
2. **避免告警疲劳**——不产生大量无法 actionable 的告警
3. **使用错误率而非错误数**——避免流量波动导致误告警
4. **设置合理阈值**——基于历史数据 P99 值，而非拍脑袋

---

## Claude 行为约束

Claude 在编写日志和监控相关代码时必须遵守：

1. **使用结构化日志**——不使用字符串拼接
2. **正确选择日志级别**——不滥用 ERROR
3. **不记录敏感信息**——密码、Token 等必须脱敏
4. **添加 trace_id 传播**——跨函数调用保持 context 传递
5. **新增 API 时提醒添加指标**——至少包含 RED 三项基础指标

---

## CLI 工具日志规范（补充）

> 本节适用于命令行工具项目，与上方服务端规范并列，不互相替代。

### CLI 日志核心原则

- **默认安静（quiet by default）**：正常运行只输出用户数据（stdout）和必要警告（stderr），不打印流程信息
- **按需调试**：通过 `--verbose` 全局 Flag 开启 INFO + DEBUG 级别输出
- **全部写 stderr**：日志（包括 INFO/DEBUG）统一写 stderr，stdout 只输出用户数据，保证 pipe 安全
- **无时间戳**：CLI 不需要时间戳（由 shell history 提供），保持输出简洁
- **不引入重型日志库**：CLI 工具使用轻量标准库实现日志，避免增加二进制体积

### CLI 日志级别

| 级别    | 前缀      | 显示条件       | 用途 |
|---------|-----------|---------------|------|
| `ERROR` | `[ERROR]` | 始终           | 不可恢复的失败 |
| `WARN`  | `[WARN]`  | 始终           | 降级行为（使用过期缓存等） |
| `INFO`  | `[INFO]`  | `--verbose` 时 | 正常流程状态（spec 加载、缓存命中等） |
| `DEBUG` | `[DEBUG]` | `--verbose` 时 | 细粒度诊断（请求 URL、响应体、认证类型） |

> 级别前缀必须大写，格式为 `[LEVEL]` 加两个空格对齐。

### CLI 敏感信息脱敏要求

Claude 在 CLI 项目中编写日志时，必须对以下内容脱敏：

| 数据类型 | 处理方式 |
|---------|---------|
| URL query 参数（access_token、apikey、token、secret、password、key） | 值替换为 `***` |
| 请求头（Authorization、Api-Key 及同类鉴权头） | 值替换为 `***` |
| 配置文件内容（API key、token 的实际值） | **永不记录**，只记录配置文件路径 |
| 环境变量值 | 只记录变量名，不记录值 |
| 响应 body | verbose 模式下记录，超过 2048 字节截断并注明总大小 |

### CLI 关键埋点要求

在 CLI 项目中，以下位置必须有日志埋点：

| 位置 | 级别 | 内容 |
|------|------|------|
| 配置文件加载 | INFO | 加载路径（不含内容） |
| .env 文件加载 | DEBUG | 文件路径 + 变量数量（不含值） |
| Spec 缓存命中 | INFO | 服务名 + 缓存年龄 |
| Spec 远程拉取 | INFO | 服务名 + 来源 URL |
| Spec 降级使用过期缓存 | WARN | 服务名 + 过期时长 + 原因 |
| 认证注入 | DEBUG | 服务名 + provider 类型（不含 key 值） |
| HTTP 请求发出 | DEBUG | Method + 脱敏后 URL + body 大小 |
| HTTP 响应接收 | DEBUG | status code + body 大小 + 耗时 |
| HTTP 响应 body | DEBUG | 截断后的响应内容 |

### CLI --verbose Flag 接入规范

```go
// 1. 在 root command 添加全局 PersistentFlag
root.PersistentFlags().Bool("verbose", false, "enable verbose output for debugging")

// 2. 必须预扫描 os.Args，不能等 cobra 解析后再读取
//    原因：config 加载、spec 加载均发生在 cobra 解析之前
verbose := containsFlag(os.Args, "--verbose")
log.Init(verbose)
```

### CLI 日志包依赖约束

- `internal/log` 必须是叶子依赖，**只能引用标准库**，不可引用项目内其他包
- 其他包可以引用 `internal/log`，禁止反向依赖
- 禁止在 `internal/log` 中引入任何第三方日志库
