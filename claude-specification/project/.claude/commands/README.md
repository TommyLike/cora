# 项目自定义命令列表

在 Claude Code 中输入 `/` 可以看到所有可用命令。

## 命令索引

| 命令              | 文件                | 说明                             |
| ----------------- | ------------------- | -------------------------------- |
| `/review`         | `review.md`         | 对当前 git 改动做全面代码 Review |
| `/test`           | `test.md`           | 为指定文件或功能描述生成单元测试 |
| `/migrate`        | `migrate.md`        | 创建数据库迁移文件               |
| `/debug`          | `debug.md`          | 系统性排查 Bug（假设→验证→修复） |
| `/release-note`   | `release-note.md`   | 从 git log 生成发布说明          |
| `/adr`            | `adr.md`            | 创建架构决策记录（ADR）          |
| `/security-check` | `security-check.md` | 对当前改动做安全专项检查         |
| `/refactor`       | `refactor.md`       | 指导安全重构（保持行为不变）     |
| `/api-design`     | `api-design.md`     | 设计 API 接口（遵循 API 规范）   |

## 如何新增命令

1. 在本目录新建 `<command-name>.md`
2. 第一行写 `# <命令描述>`（显示在 `/` 菜单中）
3. 正文写完整的 Prompt 指令
4. 提交到仓库，团队成员自动获得该命令

## 命令参数

在命令文件中用 `$ARGUMENTS` 接收用户输入：

```
用户输入：/test internal/service/order.go
$ARGUMENTS 的值：internal/service/order.go
```

## 命名规范

- 文件名全小写，使用 `-` 连接多词：`code-review.md`
- 按业务域分组时放子目录：`db/migrate.md` → `/db:migrate`
- 命令名应是动词短语，表明操作意图
