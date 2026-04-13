# 团队 Claude 规范入口

> 本文件是团队级 Claude 规范的统一入口。所有团队成员在使用 Claude Code 时，
> 应以本文件定义的规范作为基础准则。项目层和个人层规范可在此基础上扩展，但不应违背核心原则。

---

## 规范引用

Claude 在本项目中工作时，请阅读并遵循以下规范文件：

- **架构规范** → `docs/standards/architecture.md`
- **编码规范（通用）** → `docs/standards/coding/base.md`
- **语言专项规范** → `docs/standards/coding/<lang>.md`（按项目实际语言加载）
- **测试规范** → `docs/standards/testing.md`
- **安全规范** → `docs/standards/security.md`
- **文档写作规范** → `docs/standards/docs-writing.md`
- **Git 工作流规范** → `docs/standards/git-workflow.md`
- **API 设计规范** → `docs/standards/api-design.md`
- **数据库设计规范** → `docs/standards/database.md`
- **日志与可观测性规范** → `docs/standards/observability.md`
- **发布流程规范** → `docs/standards/release.md`

---

## 核心工作原则

1. **先理解，再修改**：在修改任何代码前，先读懂当前实现的意图
2. **最小改动原则**：只做被明确要求或明确必要的修改，不擅自重构周边代码
3. **安全优先**：任何涉及用户输入、外部接口、权限的代码，必须参照安全规范
4. **可验证交付**：每次修改完成后，主动运行相关测试或说明验证方式
5. **有疑问先问**：遇到需求模糊或技术判断两可时，先提问再行动

---

## Claude 行为约束

- 不自动提交代码，除非用户明确要求
- 不自动 push 到远端仓库
- 不删除文件，除非用户明确指定
- 涉及数据库 DDL、基础设施变更，必须先展示操作内容再执行
- 不绕过 pre-commit hook 或 CI 检查

---

## 项目层扩展说明

各项目的 `CLAUDE.md` 会在本文件规范基础上追加项目特有约定。
项目层规范优先级高于团队层，但不应与团队核心原则冲突。
