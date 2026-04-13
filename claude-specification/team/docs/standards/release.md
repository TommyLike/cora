# 发布流程规范

> 本文件定义团队从代码合并到生产上线的完整发布流程。
> Claude 在协助发布相关操作时（生成 CHANGELOG、打 Tag、创建 Release）应遵循本规范。

---

## 发布策略

> [团队填写] 选择发布策略：
>
> - **滚动发布（Rolling Update）**：逐批替换实例，无停机
> - **蓝绿部署（Blue-Green）**：两套环境切换，快速回滚
> - **金丝雀发布（Canary）**：先小比例放量验证，逐步全量
> - **灰度发布**：按用户/地域/比例逐步放量

---

## 发布前检查清单

每次发布前必须确认以下条件全部满足：

### 代码就绪

- [ ] PR 已合并到 main 分支
- [ ] 所有 CI 检查通过（lint + test + build）
- [ ] Code Review 已获得所需数量的 Approve
- [ ] 无未解决的 Review Comment

### 数据库就绪

- [ ] Migration 文件已合并
- [ ] Migration 已在 staging 环境验证通过
- [ ] 涉及大表变更已通知 DBA 并获得确认
- [ ] 回滚 Migration（DOWN）已验证可用

### 配置就绪

- [ ] 新增环境变量已在目标环境配置
- [ ] Feature Flag 已设置为正确状态
- [ ] 第三方服务的 API Key / 密钥已配置

### 文档就绪

- [ ] CHANGELOG 已更新
- [ ] API 文档已同步（如有接口变更）
- [ ] Runbook 已更新（如有运维变更）

### 回滚方案

- [ ] 回滚步骤已明确记录
- [ ] 回滚触发条件已定义
- [ ] 数据库回滚方案已准备（如有 Migration）

---

## 版本号与 Tag

### 版本号规范

采用 [Semantic Versioning](https://semver.org/)：

```
v<MAJOR>.<MINOR>.<PATCH>

v1.0.0  → 首个正式版本
v1.1.0  → 新增向下兼容的功能
v1.1.1  → Bug 修复
v2.0.0  → 不兼容的 API 变更
```

### 预发布版本

```
v1.2.0-rc.1   # Release Candidate
v1.2.0-beta.1 # Beta 版本
```

### Tag 操作

```bash
# 在 main 分支上打 Tag
git tag -a v1.2.0 -m "Release v1.2.0: <简要说明>"
git push origin v1.2.0
```

**规则：**

- Tag 只在 main 分支上打
- Tag 名称必须以 `v` 开头
- 每个 Tag 必须有对应的 CHANGELOG 条目
- 不删除已发布的 Tag（特殊情况除外）

---

## CHANGELOG 管理

### 格式

遵循 [Keep a Changelog](https://keepachangelog.com/) 格式：

```markdown
# Changelog

## [Unreleased]

### Added
- 新增用户邮箱验证功能 (#123)

### Changed
- 优化订单查询接口响应速度 (#456)

### Fixed
- 修复用户注册时邮箱大小写不一致的问题 (#789)

## [1.2.0] - 2024-03-15

### Added
- ...

### Breaking Changes
- 移除 /v1/users/search 接口，请使用 /v1/users?q= 代替
```

### 分类标签

| 标签 | 含义 |
| --- | --- |
| Added | 新增功能 |
| Changed | 现有功能的变更 |
| Deprecated | 即将移除的功能 |
| Removed | 已移除的功能 |
| Fixed | Bug 修复 |
| Security | 安全漏洞修复 |
| Breaking Changes | 不兼容变更（必须醒目标注） |

### 编写规则

- 每条记录关联 PR 或 Issue 编号
- 面向**使用者**写（描述影响，而非实现细节）
- Breaking Changes 必须单独标注并说明迁移方式
- `[Unreleased]` 部分在开发过程中持续维护，发布时移入对应版本

---

## 发布流程

### 标准发布流程

```
1. 确认 main 分支上的代码即待发布内容
2. 执行发布前检查清单（全部勾选）
3. 将 CHANGELOG 中 [Unreleased] 内容移入新版本号
4. 提交 CHANGELOG 变更
5. 打 Tag（git tag -a vX.Y.Z）
6. Push Tag 触发 CI/CD 发布流水线
7. 验证 staging 环境部署结果
8. 执行生产部署
9. 验证生产环境（Smoke Test）
10. 通知相关方发布完成
```

### 发布窗口

> [团队填写] 发布时间约定：
>
> - 允许发布时间段：[填写]（如工作日 10:00-16:00）
> - 禁止发布时间段：[填写]（如周五下午、节假日前一天）
> - 紧急发布（Hotfix）不受窗口限制，但需要审批

### 发布通知

发布完成后通知以下渠道：

> [团队填写] 通知渠道和模板：
>
> - Slack/飞书频道：[填写]
> - 通知内容：版本号 + 主要变更 + CHANGELOG 链接

---

## 回滚方案

### 回滚触发条件

- 核心业务错误率超过 [团队填写]%
- P99 延迟超过基线 [团队填写] 倍
- 出现数据不一致
- 用户大量反馈异常

### 回滚步骤

```
1. 确认需要回滚（参照触发条件）
2. 通知团队正在执行回滚
3. 回滚应用到上一个稳定版本
4. 如有 Migration，执行 DOWN 回滚
5. 验证回滚后服务恢复正常
6. 发送回滚通知
7. 事后分析根因，修复后重新发布
```

### 回滚的数据库考虑

- 如果新 Migration 已执行且有数据写入：需评估 DOWN Migration 对数据的影响
- 涉及不可逆数据变更的 Migration：必须在发布前准备独立的数据恢复脚本
- **原则：先回滚应用，再评估是否回滚数据库**

---

## Feature Flag

### 何时使用 Feature Flag

- 大功能需要多次 PR 合入但未完成
- 需要按比例灰度的功能
- 需要快速关闭的风险功能

### Feature Flag 规范

- 命名：`FF_<DOMAIN>_<FEATURE>`（如 `FF_ORDER_NEW_PAYMENT`）
- 每个 Feature Flag 必须有**过期日期**
- 功能全量上线稳定后，必须在 [团队填写] 个迭代内清理 Flag 代码
- 不允许 Feature Flag 嵌套超过 2 层

---

## Claude 行为约束

Claude 在协助发布相关工作时必须遵守：

1. **生成 CHANGELOG 时**遵循 Keep a Changelog 格式，关联 PR 编号
2. **打 Tag 前**提醒用户确认已完成发布检查清单
3. **不直接执行生产部署命令**——展示命令并等待用户确认
4. **生成回滚方案**时必须考虑数据库变更的回滚
5. **提醒清理过期 Feature Flag**
