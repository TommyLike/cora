# .claude/ 目录说明

本目录存放 **Claude Code 专属配置**，包括自定义命令（Commands）和钩子（Hooks）。
与 `docs/standards/` 中面向所有工具的文档规范不同，本目录只影响 Claude Code 的行为。

---

## 目录结构

```
.claude/
├── README.md                  # 本文件：Commands 和 Hooks 完整说明
├── settings.json              # 项目级 Claude Code 配置（hooks、权限等）
└── commands/                  # 自定义斜杠命令
    ├── README.md              # 命令列表索引
    ├── review.md              # /review 命令
    ├── test.md                # /test 命令
    ├── migrate.md             # /migrate 命令
    └── <domain>/              # 按业务域分组（可选）
        └── <command>.md       # /domain:command 命令
```

---

## 一、Commands（自定义命令）

### 1.1 什么是 Commands

Commands 是将常用 Prompt 封装为可复用斜杠命令的机制。

- 存储在 `.claude/commands/<name>.md`（项目级，团队共享）
- 或 `~/.claude/commands/<name>.md`（个人级，仅本机生效）
- 在 Claude Code 中通过 `/<name>` 调用
- 子目录形成命名空间：`commands/db/migrate.md` → `/db:migrate`

**适合封装的场景：**

- 需要引用多个规范文档的复合 Review 任务
- 有固定格式要求的代码生成（迁移脚本、测试文件）
- 团队统一的工作流步骤（发布前检查、故障排查流程）
- 需要项目上下文的操作（必须结合 CLAUDE.md 中的约定）

**不适合封装的场景：**

- 简单的一次性问题
- 不依赖项目上下文的通用操作

### 1.2 命令文件格式

```markdown
# 命令标题（显示在命令列表中）

命令的完整指令内容。支持 Markdown 格式。

可以引用规范文件：`docs/standards/testing.md`
可以接收参数：$ARGUMENTS
```

**关键规则：**

- 第一行 `#` 标题作为命令描述显示在 `/` 菜单中
- `$ARGUMENTS` 会被替换为用户在命令名后输入的所有文本
- 命令文件本身就是 Prompt，写法与普通对话指令相同
- 可以在命令内引用项目文件路径，Claude 会自动读取

### 1.3 命令参数使用示例

```
用户输入：/test internal/service/user.go
实际执行：命令内容中的 $ARGUMENTS 被替换为 "internal/service/user.go"
```

### 1.4 命令命名约定

| 命名                   | 调用方式    | 适用场景    |
| ---------------------- | ----------- | ----------- |
| `commands/review.md`   | `/review`   | 通用 Review |
| `commands/test.md`     | `/test`     | 生成测试    |
| `commands/migrate.md`  | `/migrate`  | 数据库迁移  |
| `commands/db/seed.md`  | `/db:seed`  | 按域分组    |
| `commands/ci/check.md` | `/ci:check` | CI 检查     |

### 1.5 内置示例命令

本目录已提供以下示例命令，可直接使用或按项目需求修改：

| 命令       | 文件                  | 功能                         |
| ---------- | --------------------- | ---------------------------- |
| `/review`  | `commands/review.md`  | 对当前改动做全面代码 Review  |
| `/test`    | `commands/test.md`    | 为指定文件或功能生成单元测试 |
| `/migrate` | `commands/migrate.md` | 创建数据库迁移文件           |

---

## 二、Hooks（钩子）

### 2.1 什么是 Hooks

Hooks 是在 Claude Code 执行特定操作时自动触发的 Shell 命令。配置在 `settings.json` 的 `hooks` 字段中。

**核心价值：**

- 自动化质量检查（Claude 改完代码，自动触发 lint）
- 防护机制（阻止 Claude 执行危险操作）
- 工作流集成（写入文件后自动通知、记录日志）

### 2.2 Hook 触发事件

| 事件           | 触发时机              | 典型用途                      |
| -------------- | --------------------- | ----------------------------- |
| `PreToolUse`   | Claude **调用工具前** | 拦截危险操作、权限检查        |
| `PostToolUse`  | Claude **调用工具后** | 自动 lint、运行测试、记录日志 |
| `Notification` | Claude 发送通知时     | 消息推送（Slack、桌面通知）   |
| `Stop`         | Claude **完成回复后** | 汇总报告、清理临时文件        |
| `SubagentStop` | 子 Agent 完成后       | 子任务完成回调                |

### 2.3 Hook 配置格式

`settings.json` 完整结构：

```json
{
  "hooks": {
    "<事件名>": [
      {
        "matcher": "<工具名或正则>",
        "hooks": [
          {
            "type": "command",
            "command": "<shell 命令>",
            "timeout": 30000
          }
        ]
      }
    ]
  }
}
```

**字段说明：**

| 字段      | 类型   | 说明                                                    |
| --------- | ------ | ------------------------------------------------------- |
| `matcher` | string | 工具名精确匹配或正则（如 `"Edit\|Write"` 匹配两个工具） |
| `type`    | string | 固定为 `"command"`                                      |
| `command` | string | 要执行的 Shell 命令，在项目根目录执行                   |
| `timeout` | number | 超时毫秒数，默认 60000（60 秒）                         |

### 2.4 可用工具名（matcher 参考）

Claude Code 内置工具名（用于 matcher）：

| 工具名   | 触发场景          |
| -------- | ----------------- |
| `Edit`   | 编辑现有文件      |
| `Write`  | 创建或覆写文件    |
| `Bash`   | 执行 Shell 命令   |
| `Read`   | 读取文件          |
| `Glob`   | 文件搜索          |
| `Grep`   | 内容搜索          |
| `mcp__*` | 任意 MCP 工具调用 |

### 2.5 Hook 退出码语义

| 退出码               | 含义                                                |
| -------------------- | --------------------------------------------------- |
| `0`                  | 成功，继续执行                                      |
| `2`（仅 PreToolUse） | **阻止**本次工具调用，Claude 收到阻止原因后重新规划 |
| 其他非零             | 执行失败，记录错误但继续（不阻止工具）              |

> `PreToolUse` 中退出码 `2` 是唯一能阻止 Claude 执行某个操作的方式。
> stdout 和 stderr 的内容会作为阻止原因反馈给 Claude。

### 2.6 Hook 中可用的环境变量

在 Hook 的 Shell 命令中可读取以下环境变量：

| 变量                 | 说明                 | 示例值                         |
| -------------------- | -------------------- | ------------------------------ |
| `CLAUDE_TOOL_NAME`   | 触发 Hook 的工具名   | `Edit`                         |
| `CLAUDE_TOOL_INPUT`  | 工具调用参数（JSON） | `{"file_path":"/src/main.go"}` |
| `CLAUDE_SESSION_ID`  | 当前会话 ID          | `abc123`                       |
| `CLAUDE_PROJECT_DIR` | 项目根目录绝对路径   | `/Users/dev/myproject`         |

> 在 Hook 命令中可用 `echo $CLAUDE_TOOL_INPUT | jq '.file_path'` 提取具体字段。

### 2.7 典型 Hook 场景示例

#### 场景 A：文件修改后自动 lint（PostToolUse）

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "make lint 2>&1 | tail -30"
          }
        ]
      }
    ]
  }
}
```

**效果：** Claude 每次编辑或创建文件后，自动运行 lint 并将结果展示给 Claude，Claude 可据此自动修正问题。

---

#### 场景 B：阻止向生产数据库执行 DROP（PreToolUse）

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo $CLAUDE_TOOL_INPUT | jq -r '.command' | grep -qiE '(drop table|truncate|delete from).*(prod|production)' && echo '危险：检测到对生产数据库的破坏性操作，已阻止' && exit 2 || exit 0"
          }
        ]
      }
    ]
  }
}
```

**效果：** 如果 Claude 尝试执行包含 DROP/TRUNCATE/DELETE 且涉及生产环境的命令，自动阻止并返回原因。

---

#### 场景 C：修改后自动运行相关测试（PostToolUse）

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "FILE=$(echo $CLAUDE_TOOL_INPUT | jq -r '.file_path'); if [[ $FILE == *.go ]] && [[ $FILE != *_test.go ]]; then PKG=$(dirname $FILE); go test ./$PKG/... 2>&1 | tail -20; fi"
          }
        ]
      }
    ]
  }
}
```

**效果：** 每次修改 Go 源文件（非测试文件），自动运行同包的测试，让 Claude 知道改动是否破坏了已有测试。

---

#### 场景 D：Claude 完成任务后发送桌面通知（Stop）

```json
{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "osascript -e 'display notification \"Claude 已完成任务\" with title \"Claude Code\"'"
          }
        ]
      }
    ]
  }
}
```

**效果：** Claude 完成回复后，macOS 桌面弹出通知（适合长时间任务）。

---

#### 场景 E：禁止 Claude 直接 push 到 main（PreToolUse）

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo $CLAUDE_TOOL_INPUT | jq -r '.command' | grep -qE 'git push.*(origin )?main' && echo '禁止直接 push 到 main，请使用 PR 流程' && exit 2 || exit 0"
          }
        ]
      }
    ]
  }
}
```

---

### 2.8 Hook 编写最佳实践

1. **保持幂等**：Hook 命令可能被多次触发，结果应一致
2. **控制输出量**：使用 `tail -N` 截断长输出，避免 Claude 上下文被日志淹没
3. **快速失败**：lint / test 命令加超时，避免 Hook 卡住整个工作流
4. **PreToolUse 慎用阻止**：只在有明确安全意义时使用 exit 2，频繁阻止会降低效率
5. **调试 Hook**：`echo $CLAUDE_TOOL_INPUT | jq .` 是排查 Hook 不触发的最简方法

---

## 三、settings.json 完整示例

以下是推荐的项目级 `settings.json` 模板，涵盖常见场景：

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "comment": "阻止直接 push 到 main/master",
            "command": "echo $CLAUDE_TOOL_INPUT | jq -r '.command' | grep -qE 'git push.*(origin )?(main|master)$' && echo '禁止直接 push 到 main/master，请创建 PR' && exit 2 || exit 0"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "comment": "文件编辑后自动 lint",
            "command": "make lint 2>&1 | tail -30",
            "timeout": 30000
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "comment": "任务完成桌面通知（macOS）",
            "command": "osascript -e 'display notification \"Claude 已完成\" with title \"Claude Code\"' 2>/dev/null || true"
          }
        ]
      }
    ]
  }
}
```

> `comment` 字段是自定义注释字段（Claude Code 会忽略未知字段），便于团队理解每条 Hook 的用途。

---

## 四、维护规范

### 提交策略

| 文件                              | 是否提交到仓库            | 原因                             |
| --------------------------------- | ------------------------- | -------------------------------- |
| `commands/*.md`                   | **是**                    | 团队共享命令，统一工作流         |
| `settings.json`（团队共享 hooks） | **是**                    | 质量门禁和安全防护应对所有人生效 |
| `settings.json`（个人偏好 hooks） | **否**，加入 `.gitignore` | 个人偏好不强制他人               |
| `README.md`                       | **是**                    | 文档随代码提交                   |

### 命令迭代流程

```
个人发现痛点 → 本地 ~/.claude/commands/ 验证 →
效果好 → 提 PR 到项目 .claude/commands/ →
经团队确认通用价值 → 提至 team-standards 仓库 →
脚手架更新，新项目自动包含
```

### 命令质量标准

一个好的 Command 应该：

- **标题清晰**：第一行标题能让人从命令列表中立即理解用途
- **引用规范**：明确引用相关规范文件，确保 Claude 按团队标准执行
- **范围明确**：说明命令的边界（做什么、不做什么）
- **可预期**：多次执行同类任务，输出格式和质量稳定
