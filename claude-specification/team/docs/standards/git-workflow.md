# Git 工作流规范

> 本文件定义团队 Git 协作流程的统一约定。
> 所有团队成员在使用 Claude Code 时，Claude 应遵循本规范进行分支、提交和合并操作。

---

## 分支策略

### 主分支

| 分支 | 用途 | 保护规则 |
| --- | --- | --- |
| `main` | 生产代码，始终可部署 | 禁止直接 push，必须通过 PR 合并 |
| `develop` | 开发主线（如采用 Git Flow） | 禁止直接 push，必须通过 PR 合并 |

> [团队填写] 选择分支模型：
>
> - **Trunk-based**：只有 `main`，短生命周期 feature 分支，频繁合并
> - **GitHub Flow**：`main` + feature 分支，PR 合并后直接部署
> - **Git Flow**：`main` + `develop` + feature/release/hotfix 分支

### 分支命名规范

```
feat/<ticket-id>-<short-desc>     # 新功能
fix/<ticket-id>-<short-desc>      # Bug 修复
hotfix/<ticket-id>-<short-desc>   # 生产紧急修复
refactor/<ticket-id>-<short-desc> # 重构（不改变行为）
chore/<short-desc>                # 构建、CI、依赖等非业务变更
docs/<short-desc>                 # 纯文档修改
```

**规则：**

- `<ticket-id>` 为任务跟踪系统的编号（如 JIRA-123）
- `<short-desc>` 使用 kebab-case，不超过 5 个单词
- 分支生命周期不超过 [团队填写] 个工作日，超期需拆分

---

## Commit 规范

### 格式

采用 [Conventional Commits](https://www.conventionalcommits.org/) 格式：

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type 类型

| Type | 含义 | 示例 |
| --- | --- | --- |
| `feat` | 新功能 | `feat(user): add email verification` |
| `fix` | Bug 修复 | `fix(order): correct price calculation` |
| `refactor` | 重构（不改变行为） | `refactor(auth): extract token service` |
| `docs` | 文档修改 | `docs: update API guide` |
| `test` | 测试相关 | `test(user): add registration edge cases` |
| `chore` | 构建、CI、依赖 | `chore: upgrade Go to 1.22` |
| `perf` | 性能优化 | `perf(query): add index for user lookup` |
| `ci` | CI/CD 变更 | `ci: add lint step to pipeline` |

### 规则

- Subject 使用英文，小写开头，不加句号，不超过 72 字符
- Body 说明 **为什么**改，而非**改了什么**
- 涉及 Breaking Change 时，在 footer 添加 `BREAKING CHANGE: <说明>`
- 每个 commit 应是一个独立的、可编译通过的变更
- 不允许出现 `fix typo`、`update`、`wip` 等无意义 commit message

---

## Pull Request 流程

### PR 创建要求

1. PR 标题遵循 Commit 规范格式（合并时作为 squash commit message）
2. PR 描述必须包含：
   - **变更内容**：1-3 句话说明做了什么
   - **关联任务**：链接到任务跟踪系统
   - **测试方式**：如何验证本次变更
   - **影响范围**：是否涉及数据库变更、配置变更、API 变更
3. 单个 PR 不超过 [团队填写] 行变更（建议 400 行），超过需拆分

### Review 要求

| 条件 | 要求 |
| --- | --- |
| 最少 Reviewer 数 | [团队填写]（建议 1-2 人） |
| CI 检查 | 必须全部通过（lint + test + build） |
| 代码覆盖率 | 新代码覆盖率不低于 [团队填写]% |
| Review 响应 SLA | 提交后 [团队填写] 小时内完成首次 Review |

### 合并策略

> [团队填写] 选择合并方式：
>
> - **Squash and Merge**（推荐）：PR 的所有 commit 合并为一个，保持主线整洁
> - **Merge Commit**：保留完整 commit 历史
> - **Rebase and Merge**：线性历史，无 merge commit

### 合并后操作

- 合并后自动删除 feature 分支
- 如需 cherry-pick 到其他分支，在 PR 中说明

---

## Hotfix 流程

紧急生产修复的特殊流程：

```
1. 从 main 创建 hotfix/<ticket-id>-<desc> 分支
2. 修复 + 测试（可缩减为最小必要测试）
3. 提 PR 到 main，标记为 urgent
4. 至少 1 人 Review 后合并
5. 立即部署
6. 如有 develop 分支，同步 cherry-pick 到 develop
7. 事后补充完整测试
```

---

## Tag 与版本号

采用 [Semantic Versioning](https://semver.org/)：

```
v<MAJOR>.<MINOR>.<PATCH>

MAJOR：不兼容的 API 变更
MINOR：向下兼容的新功能
PATCH：向下兼容的 Bug 修复
```

**规则：**

- 发布 Tag 格式：`v1.2.3`
- Pre-release 格式：`v1.2.3-rc.1`
- Tag 必须在 main 分支上打
- 每个 Tag 必须有对应的 CHANGELOG 条目

---

## Claude Code 行为约束

Claude 在执行 Git 操作时必须遵守：

1. **不直接 push 到 main/develop**（Hook 已阻止）
2. **不使用 `--force` push**，除非用户明确要求且确认影响
3. **不使用 `git add .`**，应逐个选择要暂存的文件
4. **不跳过 pre-commit hook**（不使用 `--no-verify`）
5. **Commit message 必须符合 Conventional Commits 格式**
6. **创建分支前**，先确认当前分支状态（是否有未提交改动）
