# Cora 结果展示系统设计文档

> **版本**：v1.0
> **日期**：2026-04-17
> **状态**：已接受
> **关联模块**：`internal/view/`、`internal/output/`、`internal/builder/`

---

## 1. 背景与目标

### 1.1 问题

Cora 通过 OpenAPI Spec 动态生成命令，完全不感知各服务的业务语义。这带来了展示层的困境：

- 各服务 API 返回的 JSON 字段数量、嵌套层次、命名风格差异极大
- 默认将整个 JSON 原样输出，用户（和 Agent）需要自行过滤，使用体验差
- 部分字段（如 body、description）是长文本，直接打印会淹没关键信息
- 部分字段（如 labels、assignees）是嵌套数组，默认输出可读性极低
- 无法对时间戳、布尔值等做格式友好化

### 1.2 目标

1. 引入 **View 系统**，通过声明式配置定制每个操作的输出字段和展示方式
2. 支持 **全局格式覆盖**：`--format json|yaml` 可绕过所有 View 定制，输出完整原始数据
3. View 定义分两层：**内置 View**（随二进制发布，覆盖常用操作）和**用户 View**（`views.yaml`，可扩展覆盖任意操作）
4. 对用户和 Agent 均友好：人类看格式化表格，Agent 用 `--format json/yaml` 取完整结构化数据

---

## 2. 全局输出格式

### 2.1 `--format` flag

`--format` 是 root command 上的 `PersistentFlag`，对所有子命令生效。

| 值 | 行为 |
|----|------|
| `table`（默认） | 应用 View 定义展示格式化表格；无 View 时 fallback |
| `json` | 跳过所有 View，将完整响应体 pretty-print 为 JSON 输出到 stdout |
| `yaml` | 跳过所有 View，将完整响应体转换为 YAML 输出到 stdout |

**关键原则：`--format json/yaml` 永远输出完整、未经过滤的原始响应。** View 系统只在 `--format table` 时生效，永远不裁剪 `--format json/yaml` 的输出。

### 2.2 格式转换规则

```
响应体（JSON bytes）
  ├── --format json  →  json.MarshalIndent(v, "", "  ")  →  stdout
  ├── --format yaml  →  json 反序列化 → yaml.Marshal →  stdout
  └── --format table →  进入 View 渲染流程（见第 3 节）
```

YAML 转换使用标准 `gopkg.in/yaml.v3`，JSON → interface{} → YAML Marshal，保证结构完整。

---

## 3. View 系统

### 3.1 概念层次

```
View Registry
  ├── built-in views   （Go 代码，随二进制发布）
  └── user views       （~/.config/cora/views.yaml，用户自定义）

查找键：service + resource/verb
  例：gitcode + issues/get
      forum   + topics/list
```

### 3.2 查找优先级

```
1. 用户 views.yaml 中 <service>.<resource>/<verb>    ← 最高优先级
2. 内置 built-in view 中 <service>.<resource>/<verb>
3. 全局 fallback（见 3.7 节）
```

用户 View 完全覆盖同键的内置 View（不合并 columns，整体替换）。

---

## 4. views.yaml 规格

### 4.1 文件位置

| 来源 | 路径 |
|------|------|
| 默认位置 | `~/.config/cora/views.yaml` |
| 环境变量覆盖 | `CORA_VIEWS` 指定的绝对路径 |

文件缺失时静默忽略（不报错），仅使用内置 View。

### 4.2 顶层结构

```yaml
# ~/.config/cora/views.yaml

<service-name>:
  <resource>/<verb>:
    columns:
      - <ColumnDef>
      - <ColumnDef>
      ...

  <resource>/<verb>:
    columns:
      ...
```

- `<service-name>`：与 `config.yaml` 中 `services` 的键名一致（如 `gitcode`、`forum`）
- `<resource>/<verb>`：与 Cora 命令路径一致（如 `issues/get`、`topics/list`）

### 4.3 ColumnDef 结构

```yaml
- field: <string>         # 必填。JSON 字段路径，支持点号嵌套
  label: <string>         # 可选。表头/字段名，默认自动 title-case(field)
  format: <FormatType>    # 可选。值渲染方式，默认 text
  truncate: <int>         # 可选。内容最大字符数（0=不截断）
  width: <int>            # 可选。固定列宽字符数（0=自动，仅 list 横向表格有效）
  date_fmt: <string>      # 可选。仅 format=date 时有效，Go 时间格式，默认 "2006-01-02"
  indent: <bool>          # 可选。仅 format=json 时有效，true=缩进展示，false=紧凑单行
```

### 4.4 FormatType 枚举

| 值 | 适用场景 | 渲染逻辑 |
|----|---------|---------|
| `text`（默认）| 字符串、数字、布尔 | 转为字符串，按 `truncate` 截断，布尔显示为 `true`/`false` |
| `json` | 嵌套对象、数组 | 保留原始 JSON 片段；`indent: false`（默认）紧凑单行，`indent: true` 缩进展示 |
| `date` | ISO 8601 时间戳 | 解析后按 `date_fmt` 重新格式化，解析失败则原样显示 |
| `multiline` | 长文本（body、description）| 保留换行符，按 `truncate` 字符数截断（而非按列宽折行） |

### 4.5 完整示例

```yaml
gitcode:
  issues/get:
    columns:
      - field: number
        label: "No."
      - field: title
        label: Title
        truncate: 80
      - field: state
      - field: user.login
        label: Author
      - field: assignees
        format: json        # 渲染为紧凑 JSON 数组
      - field: labels
        format: json
      - field: created_at
        label: Created
        format: date
        date_fmt: "2006-01-02"
      - field: body
        label: Description
        format: multiline
        truncate: 500

  issues/list:
    columns:
      - field: number
        label: "No."
        width: 6
      - field: title
        truncate: 50
        width: 52
      - field: state
        width: 8
      - field: user.login
        label: Author
        width: 16
      - field: created_at
        label: Created
        format: date
        width: 12

  repos/get:
    columns:
      - field: full_name
        label: Repo
      - field: description
        truncate: 80
      - field: stargazers_count
        label: Stars
      - field: language
      - field: license.name
        label: License
      - field: topics
        label: Topics
        format: json
      - field: created_at
        label: Created
        format: date

forum:
  topics/list:
    columns:
      - field: id
        width: 8
      - field: title
        truncate: 60
      - field: posts_count
        label: Posts
        width: 8
      - field: reply_count
        label: Replies
        width: 8
      - field: created_at
        label: Created
        format: date
        width: 12

  posts/list:
    columns:
      - field: id
      - field: cooked          # Discourse 的 HTML 正文
        label: Content
        format: multiline
        truncate: 200
      - field: username
        label: Author
      - field: created_at
        label: Created
        format: date
```

---

## 5. 渲染模式

### 5.1 响应类型判断

```
响应 JSON
  ├── 根节点是数组 []    →  List 模式（横向表格）
  └── 根节点是对象 {}    →  Object 模式（竖式 KV 表）
```

部分 API（如 GitCode issues list）将数组嵌套在对象字段中（如 `{"data": [...], "total": 42}`）。View 定义可通过 `root_field` 指定展开路径（见 5.3 节）。

### 5.2 Object 模式（get 类操作）——竖式 KV 表

单个对象按列定义逐行展示，左列为字段名/Label，右列为渲染后的值：

```
┌────────────┬─────────────────────────────────────────────┐
│ Field      │ Value                                       │
├────────────┼─────────────────────────────────────────────┤
│ No.        │ 1367                                        │
│ Title      │ Base-Service Sig 添加 committer             │
│ State      │ open                                        │
│ Author     │ robert-xingwang                             │
│ Labels     │ [{"name":"sig/base-service"}]               │
│ Created    │ 2025-03-01                                  │
│ Description│ sig-base-service：申请增加郜兴旺为 commiter… │
└────────────┴─────────────────────────────────────────────┘
```

- 左列宽度自动对齐到最长 Label
- 右列值超出终端宽度时自动折行（不截断 Value 列本身，`truncate` 仍生效）
- `format: multiline` 的字段在右列保留换行符，整体缩进对齐

### 5.3 List 模式（list 类操作）——横向表格

数组元素逐行展开，每个 column 是一个表格列：

```
 No.  │ Title                                              │ State │ Author           │ Created
──────┼────────────────────────────────────────────────────┼───────┼──────────────────┼───────────
 1367 │ Base-Service Sig 添加 committer                    │ open  │ robert-xingwang  │ 2025-03-01
 1366 │ openEuler 社区治理规范更新                         │ open  │ zhangsan         │ 2025-02-28
```

- 列宽优先取 `width`；未指定则按内容自动适配，超长按 `truncate` 截断后加 `…`
- `format: json` 的列（如 labels）在横向模式下渲染为紧凑单行 JSON（`indent` 无效）

### 5.4 `root_field` 处理数组包装

部分 API 将列表嵌在对象中：

```yaml
gitcode:
  issues/list:
    root_field: ""   # 空字符串=根节点本身（默认）

# 若 API 返回 {"items": [...], "total": 10}
some-service:
  things/list:
    root_field: items
    columns:
      - ...
```

`root_field` 为空时，若根节点本身是数组则直接展开；若根节点是对象但只有一个数组字段，自动探测并使用（启发式规则）；否则作为单对象用 Object 模式展示。

---

## 6. 字段路径提取规格

### 6.1 支持的路径语法

| 语法 | 示例 | 说明 |
|------|------|------|
| 顶层字段 | `number` | `root["number"]` |
| 点号嵌套 | `user.login` | `root["user"]["login"]` |
| 多层嵌套 | `license.spdx_id` | `root["license"]["spdx_id"]` |

> **v1.0 不支持数组索引**（如 `labels[0].name`），数组字段使用 `format: json` 整体展示。

### 6.2 提取结果类型处理

| JSON 类型 | `format: text` | `format: json` | 备注 |
|-----------|---------------|---------------|------|
| `string` | 原样 | `"value"` 带引号 | |
| `number` | 数字字符串 | 数字字符串 | |
| `bool` | `true`/`false` | `true`/`false` | |
| `null` | `—` | `null` | |
| `object` | `[object]`（降级提示） | `{"k":"v"}` | 推荐用 `format: json` |
| `array` | `[array]`（降级提示）| `[...]` | 推荐用 `format: json` |
| 路径不存在 | `—` | `—` | 不报错 |

---

## 7. Fallback 行为

当操作没有匹配的 View 时（内置和用户均无）：

```
--format table（无 View）
  ├── 响应是数组  →  尝试提取所有顶层 string/number 字段作为列，横向表格
  └── 响应是对象  →  提取所有顶层字段，竖式 KV 表
      （嵌套对象/数组字段用紧凑 JSON 展示）

--format json  →  完整 pretty JSON
--format yaml  →  完整 YAML
```

Fallback 不截断任何字段，确保信息完整可用。

---

## 8. 新增 / 修改文件清单

### 8.1 新建文件

```
internal/view/
  view.go       — ViewColumn / ViewConfig / Registry 类型定义
  extract.go    — 点号路径字段提取（ExtractField）
  loader.go     — 加载 views.yaml，合并 built-in，构建 Registry
  builtin.go    — 内置 View 定义（gitcode、forum 常用操作）
  render.go     — 按 ViewConfig + 响应类型选择 Object/List 模式渲染

views.example.yaml  — 示例配置（随代码提交，文档用途）
```

### 8.2 修改文件

| 文件 | 改动摘要 |
|------|---------|
| `internal/output/formatter.go` | `Print(data, format, *ViewConfig)` 增加第三参数；新增 YAML 格式输出路径 |
| `internal/executor/executor.go` | `Request` 增加 `ViewConfig *view.ViewConfig` 字段 |
| `internal/builder/command.go` | `Build` 接收 `*view.Registry`；查找 ViewConfig 后注入 `Request` |
| `cmd/cora/main.go` | 初始化 `view.Registry`（加载 views.yaml + built-in）；传给 `builder.Build` |
| `internal/config/config.go` | 新增 `ViewsFile string` 配置项（`views_file`），可指定 views.yaml 路径 |

---

## 9. 依赖关系

```
internal/view   ←  internal/output
internal/view   ←  internal/builder
internal/view   ←  cmd/cora

internal/view   只引用标准库 + gopkg.in/yaml.v3
internal/view   不引用 internal/auth / internal/executor / internal/spec（避免循环）
```

---

## 10. 配置示例与用户心智模型

### 10.1 用户自定义新操作的 View

```yaml
# ~/.config/cora/views.yaml
gitcode:
  pulls/list:                   # 尚无内置 View 的操作
    columns:
      - field: number
        label: "PR"
        width: 6
      - field: title
        truncate: 55
      - field: state
        width: 8
      - field: user.login
        label: Author
      - field: head.label       # 点号嵌套
        label: Branch
      - field: created_at
        format: date
        label: Created
```

### 10.2 用户覆盖内置 View

在 `views.yaml` 中定义相同的 `service/resource/verb` 键，整体替换内置 View：

```yaml
gitcode:
  issues/get:                   # 覆盖内置，只显示 3 个字段
    columns:
      - field: number
      - field: title
      - field: state
```

### 10.3 Agent 使用模式

```bash
# Agent 取完整结构化数据（不受 View 影响）
cora gitcode issues get --owner openeuler --repo community --number 1367 --format json

# Agent 取 YAML（部分工具链更易解析）
cora gitcode issues list --owner openeuler --repo community --format yaml

# 人类阅读友好输出（应用 View）
cora gitcode issues get --owner openeuler --repo community --number 1367
```

---

## 11. 关键设计决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| views 配置文件 | 独立 `views.yaml` | 服务多时不污染 config.yaml；关注点分离 |
| 单对象展示方式 | 竖式 KV 表（方案 B） | 字段多时易读；长文本字段（body）不被截断为一列 |
| List 探测方式 | 根节点类型 + `root_field` | 兼容标准数组响应和包装对象响应 |
| `--format json/yaml` 语义 | 完整原始输出，绕过 View | 保证 Agent 永远能取到完整数据 |
| View 覆盖策略 | 整体替换（非合并） | 行为可预测，避免部分覆盖产生歧义 |
| 数组索引支持 | v1.0 不支持 | 简化实现；数组字段用 `format: json` 覆盖 |
| YAML 输出 | JSON→interface{}→YAML | 无需维护独立 YAML 解析路径；结构等价 |
