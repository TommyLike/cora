# CLI 设计模式参考：googleworkspace/cli 分析

> **来源项目**：https://github.com/googleworkspace/cli
> **分析目的**：为 cora 提炼可借鉴的多服务聚合 CLI 设计模式

---

## 1. 整体架构模式

### 1.1 Discovery 驱动的动态命令生成

**模式描述**：不硬编码命令树，而是通过服务元数据（Discovery Document）在运行时动态构建命令。

```
gws <service> <resource> [sub-resource] <method> [flags]
```

**核心优势**：
- 新 API 上线后 CLI 自动获得对应命令，无需代码变更
- 单份通用代码服务 15+ 个 Google Workspace 服务
- 命令树结构由 API 定义驱动，保持与服务端一致

**对 cora 的启发**：
- 邮件列表、会议、Issue 等服务各自维护服务描述文件（类似 OpenAPI/schema）
- CLI 读取这些描述文件动态注册命令，新增服务只需添加 schema，无需修改核心逻辑
- 可用 JSON/YAML schema 描述每个社区服务的资源和操作

---

### 1.2 Helper Trait 模式（服务特化扩展点）

**模式描述**：在通用命令框架之上，提供一个轻量扩展接口，仅在通用方式无法满足时才接入。

```go
// 概念模型（Go 风格伪代码）
type Helper interface {
    InjectCommands(cmd *Command, schema *ServiceSchema) *Command
    Handle(ctx context.Context, args []string) error
    IsHelperOnly() bool
}
```

**扩展点的使用标准（严格控制）**：

| 合理使用场景 | 不应使用场景 |
|-------------|-------------|
| 多步骤 API 编排（链式调用） | 单个 API 的简单包装 |
| 格式转换（如 Markdown → HTML） | 仅为参数取别名 |
| 复杂请求体构建（如附件处理） | 无限叠加 flag |
| 可恢复上传等特殊协议 | 重复 Discovery 已有参数 |

**判断标准（Litmus Test）**：若用户通过 `--params '{"key":"value"}'` + 通用输出过滤能完成任务，则不需要 Helper。

**对 cora 的启发**：
- 默认命令由服务 schema 自动生成；
- 仅当需要跨服务编排（如：从 Issue 创建会议日程）时，才实现专用 Helper；
- 避免为每个常用操作都创建快捷命令——保持核心层薄。

---

## 2. 多服务聚合模式

### 2.1 服务注册表

**模式描述**：维护一个中央服务注册表，映射服务名称到对应的 Helper 实例和 schema 来源。

```
community <service> <resource> <method> [flags]
community mail list --filter "subject:release"
community meeting list --upcoming
community issue list --repo cncf/xxx --state open
```

**注册表设计要点**：
- 服务名称支持别名（如 `mail` = `mailing-list`）
- 按需加载服务 schema（懒加载，提升启动速度）
- schema 可来自本地文件、内嵌资源或远端 URL

### 2.2 统一命令结构

所有服务遵循相同的命令模式，降低学习曲线：

```
<binary> <service> <resource> <verb> [flags]
         ↑         ↑           ↑
         注册的服务  资源类型    标准操作(list/get/create/delete)
```

---

## 3. 认证与凭证管理

### 3.1 多层凭证优先级

按如下优先级查找认证凭证（高优先级覆盖低优先级）：

```
1. 环境变量 TOKEN（最高优先级，适合 CI/CD）
   ↓
2. 环境变量指定的凭证文件路径
   ↓
3. ~/.config/<cli>/credentials.enc（加密存储，默认）
   ↓
4. ~/.config/<cli>/credentials.json（明文，降级）
   ↓
5. Application Default Credentials / 平台默认认证
```

**对 cora 的启发**：社区服务可能有多种认证方式（GitHub Token、邮件列表 API Key、Slack Bot Token），统一用分层查找逻辑处理，用户只需设置一次。

### 3.2 加密存储 + 平台 Keyring

**模式描述**：
- 默认使用 AES-256-GCM 加密存储凭证文件
- 优先使用 OS 原生 Keyring（macOS Keychain、Windows Credential Store）
- Linux 下保留文件加密备份（容器环境缺乏 Keyring）
- 原子写入（先写临时文件再 rename）+ 0600 权限保护

### 3.3 多服务统一 Auth 命令

```
community auth login [--service github|slack|...]
community auth logout
community auth status
community auth token --service github
```

不同服务的认证流程各异（OAuth、API Key、Token），但对用户暴露统一的 `auth` 命令入口。

---

## 4. 配置管理

### 4.1 配置分层

```
默认配置（内置）
    ↓ 覆盖
系统级配置文件（/etc/cora/config.yaml）
    ↓ 覆盖
用户配置文件（~/.config/cora/config.yaml）
    ↓ 覆盖
项目配置文件（./.cora.yaml）
    ↓ 覆盖
环境变量（CORA_*）
    ↓ 覆盖
命令行 flags（最高优先级）
```

### 4.2 配置目录结构

```
~/.config/cora/
├── config.yaml          # 全局配置（服务端点、偏好设置）
├── credentials.enc      # 加密凭证
├── cache/               # 服务 schema 缓存、API 响应缓存
│   ├── mail-schema.json
│   └── issue-schema.json
└── profiles/            # 多环境配置（dev/staging/prod）
    ├── default.yaml
    └── staging.yaml
```

**Schema 缓存**：本地缓存服务描述文件，加速命令树构建，避免每次调用都请求远端。

---

## 5. 输出格式化

### 5.1 多格式输出

所有命令支持统一的 `--format` flag：

```
--format table   # 默认，终端友好的表格展示
--format json    # JSON，适合脚本和 LLM 消费
--format yaml    # YAML，可读性好
--format csv     # CSV，适合导出到 Excel/表格
```

### 5.2 智能数据提取

对于 API 返回的嵌套对象，自动识别列表数据：
- 跳过元数据字段（`nextPageToken`、`kind`、下划线前缀字段）
- 自动找到第一个非空数组字段作为表格数据源
- 嵌套对象以点号路径展示（如 `author.name`）

### 5.3 分页感知格式化

```
community mail list --all           # 自动翻页，输出合并
community mail list --page-token X  # 手动分页
```

- JSON 模式：自动合并多页为单一 JSON 数组，保证脚本可解析
- Table 模式：跨页不重复输出表头
- 遵守 `NO_COLOR` 环境变量（管道场景自动禁用颜色）

---

## 6. 错误处理

### 6.1 结构化错误分类与退出码

| 错误类型        | 退出码 | 场景               |
|-------------|-----|------------------|
| API 错误      | 1   | 服务端返回错误          |
| 认证错误        | 2   | 未登录或 Token 过期    |
| 参数校验错误      | 3   | 用户输入有误           |
| Schema 获取失败 | 4   | 无法连接服务或加载 schema |
| 其他错误        | 5   | 未分类错误            |

**对 CI/CD 集成的价值**：脚本可通过退出码区分错误类型，而非解析错误文本。

### 6.2 双通道输出策略

```
stdout: 机器可读的结构化数据（JSON 格式错误详情）
stderr: 人类可读的有色错误摘要 + 操作建议
```

```
# 示例 stderr 输出
[Auth Error] Token expired for service 'github'
→ Run: community auth login --service github
→ Or set: COMMUNITY_CLI_GITHUB_TOKEN=<token>
```

### 6.3 上下文化错误建议

常见错误附加具体修复建议：
- Token 过期 → 提示重新登录命令
- 权限不足 → 提示需要的权限范围
- 网络错误 → 显示 HTTP 状态码和响应体
- 服务未配置 → 提示初始化命令

### 6.4 终端安全输出

对所有输出内容进行终端转义字符清理：
- 过滤 ASCII 控制字符（0x00–0x1F, 0x7F）
- 保留换行和 Tab
- 防止终端注入攻击（重要：尤其当输出来自外部服务时）

---

## 7. 安全与输入校验

### 7.1 严格输入校验（面向自动化场景）

对所有外部输入进行防御性校验：
- **文件路径**：拒绝绝对路径、`../` 遍历、CWD 外的符号链接、控制字符
- **URL 参数**：自动进行 percent-encoding
- **资源标识符**：格式校验防止注入攻击

**设计原则**：环境变量是受信输入；CLI 参数不可信，必须校验。

### 7.2 Dry-Run 支持

```
community mail send --to list@cncf.io --subject "..." --dry-run
```

所有写操作支持 `--dry-run` flag，预览将执行的 API 请求，不实际发送。

---

## 8. 用户体验设计

### 8.1 渐进式复杂度披露

```
# 简单场景：开箱即用
community issue list --repo cncf/xxx

# 进阶场景：传入任意参数 JSON
community issue list --repo cncf/xxx --params '{"state":"open","labels":"help-wanted"}'

# 高级场景：原始 API 调用 + 管道过滤
community issue list --repo cncf/xxx --format json | jq '.[] | select(.comments > 5)'
```

**设计原则**：常用操作用 flag 暴露，长尾需求通过 `--params` JSON 透传，避免 flag 爆炸。

### 8.2 帮助文本中的示例

每个命令在 `--help` 中包含 2-3 个实际可运行的示例：

```
community mail list --help

Usage: community mail list [flags]

Flags:
  --filter string   Filter expression (e.g. "subject:release AND after:2024-01")
  --limit int       Max results (default 20)
  --format string   Output format: table|json|yaml|csv (default "table")

Examples:
  community mail list --filter "subject:release"
  community mail list --limit 100 --format json | jq '.[].subject'
  community mail list --filter "from:user@example.com" --format csv > export.csv
```

### 8.3 交互式初始化向导

首次运行或执行 `community init` 时，引导用户完成服务配置：

```
$ community init
? Which services do you want to configure?
  ✓ GitHub Issues
  ✓ Mailing List (Mailman/Google Groups)
  ○ Meeting Calendar
  ○ Slack

? GitHub Token: ****
? Mailing list API endpoint: https://lists.example.org/api/v1
...
✓ Configuration saved to ~/.config/cora/config.yaml
```

---

## 9. 可扩展性设计

### 9.1 Recipes / Workflow 层

在基础命令之上，提供预构建的跨服务工作流：

```
community recipe issue-to-meeting    # 从 Issue 创建会议邀请
community recipe meeting-summary     # 会议结束后发邮件列表摘要
community recipe release-announce    # 打 Tag 后自动发邮件列表公告
```

这是一个**独立的编排层**，与底层命令完全解耦，可以独立版本化和分发。

### 9.2 输出管道友好性

所有命令输出保证：
- JSON 模式输出合法的 JSON（可直接 `jq` 处理）
- Table 模式在非 TTY（管道）时自动降级为无颜色纯文本
- 错误信息只输出到 stderr，不污染 stdout 的数据流

---

## 10. 关键设计决策汇总

| 决策 | 原因 |
|------|------|
| Discovery/Schema 驱动命令 | 服务增加时零代码改动；命令结构始终与 API 同步 |
| Helper 仅用于编排场景 | 防止核心层膨胀；保持通用框架复用性 |
| 分层凭证查找 | 同时兼容开发者本地环境和 CI/CD 无头环境 |
| AES-GCM 加密 + OS Keyring | 安全默认（加密）+ 跨平台兼容（文件降级） |
| 多格式输出统一由框架处理 | 命令开发者只关注数据逻辑，无需处理格式化 |
| 退出码分类 | 脚本/CI 可程序化处理不同错误类型 |
| stdout/stderr 分离 | 数据流与诊断信息解耦，管道场景不污染数据 |
| `--params` JSON 透传 | 避免为每个 API 参数添加 flag，保持 CLI 精简 |
| Dry-Run 支持 | 降低自动化场景的操作风险 |
| Recipe 工作流独立分层 | 跨服务编排与底层命令解耦，独立演进 |

---

## 附：cora 适用性评估

| 模式 | 适用性 | 说明 |
|------|--------|------|
| Schema 驱动命令 | ★★★★★ | 社区服务（邮件、会议、Issue）各有 API，可用 OpenAPI 描述 |
| Helper 扩展点 | ★★★★☆ | 跨服务场景（如 Issue → 会议）非常需要 |
| 分层凭证管理 | ★★★★★ | 社区用户横跨个人本地和 CI 场景 |
| 多格式输出 | ★★★★★ | 社区开发者习惯用脚本处理数据 |
| Recipes 工作流 | ★★★★☆ | 社区高频工作流（发版公告、会议纪要分发）非常适合 |
| Dry-Run | ★★★☆☆ | 写操作（发邮件、创建事件）时有价值 |
| Discovery 动态加载 | ★★★☆☆ | 初期可用静态注册，服务多了再考虑动态发现 |
