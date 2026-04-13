# 生成发布说明

根据 Git 提交历史，生成本次发布的 Release Note。

## 执行步骤

### 1. 获取变更范围

确定上次发布的 Tag 和当前 HEAD 之间的所有变更：

```bash
# 获取最新 Tag
git describe --tags --abbrev=0

# 查看自上次 Tag 以来的所有 commit
git log <上次Tag>..HEAD --oneline --no-merges
```

如果用户指定了范围，使用 $ARGUMENTS 作为 git log 的范围参数。

### 2. 分类整理

按 Conventional Commits 类型分类，使用 Keep a Changelog 格式输出：

```markdown
## [版本号] - YYYY-MM-DD

### Added
- 新增 xxx 功能 (#PR编号)

### Changed
- 优化 xxx 行为 (#PR编号)

### Fixed
- 修复 xxx 问题 (#PR编号)

### Breaking Changes
- 移除 xxx 接口，请使用 yyy 代替 (#PR编号)
```

### 3. 输出规则

- 每条记录关联 PR 编号或 commit hash
- 面向使用者而非开发者描述（说影响，不说实现）
- Breaking Changes 必须单独列出，并说明迁移方法
- 忽略 `chore`、`ci`、`style` 类型的 commit（不影响功能）
- 如果 commit message 不规范，根据代码变更内容推断分类
