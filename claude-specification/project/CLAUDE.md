# CORA Claude 规范入口

> **使用说明：** 本文件是项目级 CLAUDE.md 模板。
> 复制到项目根目录后，替换所有 `[占位符]` 内容，删除不适用的章节。
> 本文件优先级高于团队规范，与团队规范冲突时以本文件为准。

---

## 团队规范引用

本项目遵循团队基础规范，使用时请同时参考：

```
# 团队规范位于（clone team-standards 仓库后的路径）：
~/team-standards/docs/standards/
```

> [操作说明] 建议将团队规范仓库 clone 到固定位置，
> 或由脚手架在本地生成软链接，以便 Claude 可以直接引用。

---

## 项目基础信息

- **项目名称：** CORA
- **主要语言：** Go
- **架构风格：** 单体，命令行
- **所属团队：** opensource-infrastructure

---

## 项目规范文件索引

| 规范类型 | 文件路径                          | 状态                     |
| -------- | --------------------------------- | ------------------------ |
| 构建规范 | `docs/standards/build.md`         | 必填                     |
| 项目架构 | `docs/standards/architecture.md`  | 可选（覆盖团队架构规范） |
| 语言专项 | `docs/standards/coding/<lang>.md` | 可选（覆盖团队语言规范） |
| API 规范 | `openapi.yaml` / `proto/`         | 按需                     |

---

## 项目特有约定

### 技术栈约束

>  本项目实际使用的技术栈和版本
>
> - Go 1.22，使用 gin 框架
> - PostgreSQL 16，使用 gorm ORM
> - Redis 7（缓存和分布式锁）
> - 消息队列：Kafka（Topic 命名见 `docs/standards/build.md`）

### 目录结构

> [填写] 本项目的实际目录结构，例如：
>
> ```
> .
> ├── cmd/server/        # HTTP 服务入口
> ├── internal/
> │   ├── domain/        # 业务实体
> │   ├── service/       # 业务逻辑
> │   ├── repository/    # 数据访问
> │   └── handler/       # HTTP handler
> ├── migrations/        # 数据库迁移
> ├── docs/
> └── Makefile
> ```

### 代码生成

> [填写] 本项目是否有代码生成，例如：
>
> - protobuf 生成：`make proto`（需安装 protoc + protoc-gen-go）
> - mock 生成：`make mock`（使用 mockgen）
> - 修改 proto / interface 后，必须重新运行生成命令

### 特殊业务约束

> [填写] 本项目特有的业务规则，Claude 需要知道的背景，例如：
>
> - 订单状态机：状态只能单向流转，不可回退（见 `docs/order-state-machine.md`）
> - 多租户架构：所有查询必须携带 `tenant_id` 过滤条件
> - 时区处理：所有时间存储为 UTC，展示层转换为用户时区

---

## Claude 工作指引

### 常用命令

> [填写] 项目常用开发命令，Claude 在修改代码后应知道如何验证：
>
> ```bash
> make dev        # 启动开发服务
> make test       # 运行所有测试
> make lint       # 代码检查
> make build      # 编译构建
> make migrate    # 执行数据库迁移
> ```

### 验证方式

完成修改后，Claude 应提示用户执行：

1. `make lint` — 确保代码风格合规
2. `make test` — 确保测试通过
3. 如涉及 API 变更，检查 `openapi.yaml` 是否同步更新

### 禁止事项（项目特有）

> [填写] 本项目特有的禁止行为，例如：
>
> - 禁止直接修改 `migrations/` 目录下已存在的迁移文件
> - 禁止在 `internal/` 外部引用 `internal/` 内的包
> - 修改数据库 Schema 前，必须先与 DBA 确认

---

## 环境配置

> [填写] 开发环境配置说明：
>
> - 配置文件：`.env.local`（本地）、`.env.example`（模板，已提交）
> - 必填环境变量说明见 `docs/standards/build.md`
> - 本地启动依赖：Docker（用于启动 PostgreSQL + Redis）

---

## 相关资源

> [填写] 项目相关文档和工具链接：
>
> - 需求文档：[链接]
> - API 文档：[链接]
> - 监控看板：[链接]
> - CI/CD 流水线：[链接]
> - On-call Runbook：`docs/runbook.md`
