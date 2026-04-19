# Smoke Test Runner 设计文档

> **状态**：设计评审中  
> **日期**：2026-04-19  
> **目标**：对 cora 所有 service 子命令做持续可用性与体验巡检

---

## 1. 背景与目标

cora 通过 OpenAPI spec 动态生成命令，已接入 GitCode、Etherpad、Forum（Discourse）等多个服务。由于命令结构和 API 调用是运行时生成的，现有单元测试无法覆盖"真实 API 是否可达、返回结构是否符合预期、view 定制字段是否正确渲染"等端到端问题。

**目标：**

1. **可用性**：命令能否跑通，API 是否返回 2xx，退出码是否为 0
2. **正确性**：返回数据中是否包含预期字段和结构
3. **体验**：table 格式下定制的 view 列是否正确渲染，数据是否非空

**不在范围内：**

- 性能压测
- 写操作的数据一致性校验（巡检默认以只读为主，写操作需在 scenario 中显式标注并在 teardown 中清理）

---

## 2. 整体架构

```
cora/
├── cmd/
│   └── smoke/
│       └── main.go              ← 独立 Runner 二进制入口
├── internal/
│   └── smoke/
│       ├── loader.go            ← 读取 & 校验 scenario YAML 文件
│       ├── runner.go            ← 执行场景（子进程调用 cora）
│       ├── assertion.go         ← 断言评估逻辑
│       └── report.go            ← 生成控制台输出 + HTML 报告
├── scenarios/                   ← 场景文件目录（按 service 分组）
│   ├── gitcode/
│   │   ├── issues-list.yaml
│   │   └── issues-get.yaml
│   ├── forum/
│   │   └── posts-list.yaml
│   └── etherpad/
│       └── pad-list.yaml
├── config/
│   └── smoke-config.yaml        ← Runner 专用 cora 配置（指向测试环境）
└── .github/
    └── workflows/
        └── smoke.yml            ← GitHub Actions 定时触发
```

**核心设计原则：**

- Runner 与 cora 内部包**完全解耦**，只通过子进程调用 cora 二进制，测试真实的端到端链路（CLI 解析 → spec 加载 → auth 注入 → HTTP 调用 → 格式化输出）
- 场景文件是**声明式 YAML**，增加新服务只需添加文件，无需修改 Runner 代码
- HTML 报告为**自包含单文件**（CSS/JS 全部内联），可直接作为 GitHub Actions artifact 查看

---

## 3. 场景文件格式

每个 YAML 文件描述一条巡检场景。

### 3.1 字段定义

```yaml
name: "GitCode · issues list"          # 报告中显示的名称（必填）
service: gitcode                        # 对应 cora 的 <service> 参数（必填）
args:                                   # cora 子命令及参数列表（必填）
  - issues
  - list
  - --owner
  - cncf-infra
  - --repo
  - smoke-test
format: table                           # table | json | yaml，默认 table
timeout_ms: 5000                        # 单场景超时，默认 10000ms
skip: false                             # true 时跳过该场景，报告显示 SKIP
skip_reason: "staging 环境暂不支持"      # skip 原因（可选）
assertions:                             # 断言列表（按顺序评估，全部通过才算 PASS）
  - type: exit_code
    value: 0
```

### 3.2 断言类型完整列表

| 类型 | 参数 | 说明 |
|------|------|------|
| `exit_code` | `value: int` | 退出码等于预期值 |
| `response_time_lt` | `value: int`（ms） | 执行耗时 < N ms |
| `stdout_not_empty` | — | stdout 非空 |
| `stdout_contains` | `value: string` | stdout 包含指定字符串 |
| `stdout_not_contains` | `value: string` | stdout 不包含指定字符串 |
| `stderr_not_contains` | `value: string` | stderr 不包含指定字符串（常用于检查无 ERROR） |
| `table_has_columns` | `values: []string` | table 格式的列头包含所有指定列名 |
| `table_row_count_gte` | `value: int` | table 数据行数 ≥ N |
| `json_has_keys` | `values: []string` | JSON 根对象包含所有指定 key |
| `json_key_not_empty` | `key: string` | 指定 JSON key 的值非空字符串 / 非零 / 非 null |

### 3.3 场景示例

**只读列表（table 格式，检查 view 定制）：**

```yaml
name: "GitCode · issues list"
service: gitcode
args: ["issues", "list", "--owner", "cncf-infra", "--repo", "smoke-test"]
format: table
timeout_ms: 5000
assertions:
  - type: exit_code
    value: 0
  - type: response_time_lt
    value: 3000
  - type: table_has_columns
    values: ["Number", "Title", "State"]
  - type: table_row_count_gte
    value: 1
  - type: stderr_not_contains
    value: "ERROR"
```

**JSON 格式详情（检查字段结构）：**

```yaml
name: "GitCode · issues get (JSON)"
service: gitcode
args: ["issues", "get", "--owner", "cncf-infra", "--repo", "smoke-test", "--number", "1"]
format: json
timeout_ms: 5000
assertions:
  - type: exit_code
    value: 0
  - type: json_has_keys
    values: ["title", "state", "number"]
  - type: json_key_not_empty
    key: "title"
```

**Forum 帖子列表：**

```yaml
name: "Forum · posts list"
service: forum
args: ["posts", "list"]
format: table
timeout_ms: 8000
assertions:
  - type: exit_code
    value: 0
  - type: table_has_columns
    values: ["ID", "Username", "Reads"]
  - type: table_row_count_gte
    value: 1
```

---

## 4. Runner 程序设计

### 4.1 命令行接口

```
smoke-runner [flags]

Flags:
  --cora-bin       string   cora 二进制路径（默认 ./bin/cora）
  --config         string   cora 配置文件路径（默认 ./config/smoke-config.yaml）
  --scenarios-dir  string   场景文件目录（默认 ./scenarios）
  --report-dir     string   报告输出目录（默认 ./smoke-report）
  --parallel       int      并发执行数（默认 1，顺序执行）
  --timeout        duration 全局超时兜底（默认 5m）
  --filter         string   只运行名称包含该字符串的场景（用于调试）
  --verbose        bool     打印每个场景的完整 stdout/stderr
```

### 4.2 执行流程

```
1. 扫描 --scenarios-dir，递归收集所有 .yaml 文件
2. 解析 & 校验每个 scenario（字段类型、断言格式）
3. 按 service 分组，顺序（或并发）执行每个场景：
   a. 构造命令：cora --config <smoke-config> --format <format> <service> <args...>
   b. 启动子进程，设置 timeout
   c. 捕获 stdout、stderr、exit code、实际耗时
   d. 逐条评估断言，记录每条的 pass/fail 及实际值
4. 汇总所有结果
5. 输出控制台摘要
6. 生成 HTML 报告到 --report-dir/report.html
7. 若有任意 FAIL → 以退出码 1 退出；全部 PASS/SKIP → 退出码 0
```

### 4.3 内部数据结构

```go
// ScenarioResult 记录单个场景的完整执行结果
type ScenarioResult struct {
    Scenario        Scenario
    Status          Status          // PASS | FAIL | SKIP | TIMEOUT | ERROR
    DurationMs      int64
    Stdout          string
    Stderr          string
    ExitCode        int
    AssertionResults []AssertionResult
    Error           string          // 子进程启动失败等非断言错误
}

type AssertionResult struct {
    Assertion  Assertion
    Passed     bool
    Actual     string   // 实际值，用于报告展示
    Message    string   // 失败时的描述，如 "expected < 3000ms, got 5123ms"
}

type Status string
const (
    StatusPass    Status = "PASS"
    StatusFail    Status = "FAIL"
    StatusSkip    Status = "SKIP"
    StatusTimeout Status = "TIMEOUT"
    StatusError   Status = "ERROR"   // 子进程无法启动等
)
```

---

## 5. HTML 报告设计

### 5.1 结构

```
┌──────────────────────────────────────────────────────────┐
│  🔍 CORA Smoke Test Report                               │
│  Generated: 2026-04-19 10:30:00 | Duration: 12.3s       │
│                                                          │
│  ✅ 6 Passed   ❌ 2 Failed   ⏭ 1 Skipped               │  ← 顶部总览卡片
└──────────────────────────────────────────────────────────┘

┌── gitcode ─────────────────────── 4/4 ✅ ───────────────┐
│  ✅ GitCode · issues list          823ms                 │
│  ✅ GitCode · issues get (JSON)    412ms                 │
│  ✅ GitCode · repos list           634ms                 │
│  ✅ GitCode · repos get            390ms                 │
└──────────────────────────────────────────────────────────┘

┌── forum ───────────────────────── 1/2 ❌ ───────────────┐
│  ✅ Forum · posts list             1203ms                │
│  ❌ Forum · topics list            5001ms  [展开 ▼]      │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Assertions                                       │   │
│  │  ✅ exit_code = 0                                │   │
│  │  ❌ response_time_lt 3000ms — actual: 5001ms     │   │
│  │  ✅ table_has_columns: ID, Title, Views          │   │
│  │                                                  │   │
│  │ Stdout                                           │   │
│  │  ┌──────────────────────────────────────────┐   │   │
│  │  │ ID  Title              Views              │   │   │
│  │  │ 42  Release v1.2.0    128                │   │   │
│  │  └──────────────────────────────────────────┘   │   │
│  │                                                  │   │
│  │ Stderr                                           │   │
│  │  (empty)                                         │   │
│  └──────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────┘

┌── etherpad ────────────────────── SKIP ─────────────────┐
│  ⏭ Etherpad · pad list    staging 环境暂不支持           │
└──────────────────────────────────────────────────────────┘
```

### 5.2 技术要求

- **自包含**：所有 CSS / JS 内联在单个 HTML 文件，无需网络请求，可直接作为 artifact 下载打开
- **可折叠**：每个失败场景默认展开，PASS 场景默认收起，点击可切换
- **颜色编码**：绿（PASS）/ 红（FAIL / TIMEOUT）/ 灰（SKIP）/ 橙（ERROR）
- **失败优先**：FAIL 场景排在每个 service 分组的最前面
- 报告顶部显示运行时间戳、cora 版本、配置文件路径

---

## 6. 测试环境配置

Runner 使用独立的 cora 配置文件，与用户本地配置完全隔离：

```yaml
# config/smoke-config.yaml
# 专用于 smoke test 的测试环境配置
# 敏感值通过环境变量注入，不提交到仓库

services:
  gitcode:
    base_url: https://api.gitcode.com
    auth:
      gitcode:
        access_token: ${SMOKE_GITCODE_TOKEN}

  forum:
    spec_url: assets/openapi/forum/openapi.json
    base_url: ${SMOKE_FORUM_URL}          # 测试 Discourse 实例
    auth:
      discourse:
        api_key: ${SMOKE_FORUM_API_KEY}
        api_username: ${SMOKE_FORUM_USERNAME}

  etherpad:
    base_url: ${SMOKE_ETHERPAD_URL}
    auth:
      etherpad:
        api_key: ${SMOKE_ETHERPAD_API_KEY}
```

敏感值约定：环境变量以 `SMOKE_` 前缀区分，在 CI 中通过 Secrets 注入，本地开发通过 `.env` 文件加载（已加入 `.gitignore`）。

---

## 7. CI 集成（GitHub Actions）

```yaml
# .github/workflows/smoke.yml.disable
name: Smoke Tests

on:
  schedule:
    - cron: "0 2 * * *"   # 每天 UTC 02:00 运行
  workflow_dispatch:       # 支持手动触发

jobs:
  smoke:
    name: Smoke Test
    runs-on: ubuntu-latest   # 或 self-hosted（测试环境在内网时）
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build cora and smoke-runner
        run: |
          make build-prod
          go build -o bin/smoke-runner ./cmd/smoke

      - name: Run smoke tests
        env:
          SMOKE_GITCODE_TOKEN: ${{ secrets.SMOKE_GITCODE_TOKEN }}
          SMOKE_FORUM_URL: ${{ secrets.SMOKE_FORUM_URL }}
          SMOKE_FORUM_API_KEY: ${{ secrets.SMOKE_FORUM_API_KEY }}
          SMOKE_FORUM_USERNAME: ${{ secrets.SMOKE_FORUM_USERNAME }}
          SMOKE_ETHERPAD_URL: ${{ secrets.SMOKE_ETHERPAD_URL }}
          SMOKE_ETHERPAD_API_KEY: ${{ secrets.SMOKE_ETHERPAD_API_KEY }}
        run: |
          ./bin/smoke-runner \
            --cora-bin ./bin/cora \
            --config ./config/smoke-config.yaml \
            --scenarios-dir ./scenarios \
            --report-dir ./smoke-report

      - name: Upload HTML report
        uses: actions/upload-artifact@v4
        if: always()    # 失败时也上传，方便排查
        with:
          name: smoke-report-${{ github.run_id }}
          path: smoke-report/report.html
          retention-days: 30
```

---

## 8. Makefile 集成

```makefile
.PHONY: smoke-build
smoke-build:
	go build -o bin/smoke-runner ./cmd/smoke

.PHONY: smoke
smoke: build-prod smoke-build
	./bin/smoke-runner \
		--cora-bin ./bin/cora \
		--config ./config/smoke-config.yaml \
		--scenarios-dir ./scenarios \
		--report-dir ./smoke-report
	@echo "Report: ./smoke-report/report.html"

.PHONY: smoke-filter
smoke-filter: build-prod smoke-build
	./bin/smoke-runner \
		--cora-bin ./bin/cora \
		--config ./config/smoke-config.yaml \
		--scenarios-dir ./scenarios \
		--filter "$(FILTER)"
```

本地运行：

```bash
# 全量巡检
make smoke

# 只跑 gitcode 相关场景（调试用）
make smoke-filter FILTER=gitcode
```

---

## 9. 扩展性说明

**接入新 service：**
1. 在 `config/smoke-config.yaml` 添加 service 配置
2. 在 `scenarios/<service>/` 下新增 YAML 场景文件
3. 无需修改任何 Go 代码

**新增断言类型：**
在 `internal/smoke/assertion.go` 中实现新的 `evaluate` 分支，YAML 中即可直接使用。

**从定时任务迁移到 PR 触发：**
在 `smoke.yml` 的 `on:` 中追加 `pull_request` trigger，无需其他改动。
