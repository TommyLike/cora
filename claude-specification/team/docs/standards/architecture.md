# 架构规范

> 本文件定义团队级架构原则与约束，适用于所有项目。
> 项目可在 `docs/standards/architecture.md` 中补充项目特有架构说明。

---

## 1. 架构风格

### 1.1 整体风格

> [团队填写] 描述团队默认采用的架构风格，例如：
>
> - 微服务架构 / 单体架构 / Modular Monolith
> - 是否使用 DDD（领域驱动设计）
> - 是否有标准的分层模型（如 Controller → Service → Repository）

```
示例：
- 默认采用分层架构：Handler → Service → Repository → Database
- 新服务优先考虑 Modular Monolith，达到明确拆分信号后再拆微服务
- 拆分信号：独立部署需求、团队边界、差异化扩缩容需求
```

### 1.2 服务边界原则

> [团队填写] 定义如何划定服务/模块边界，例如：
>
> - 按业务域划分，而非按技术层划分
> - 服务之间通过接口定义边界，不共享内部实现

---

## 2. 模块与依赖规范

### 2.1 依赖方向

> [团队填写] 定义依赖规则，例如：
>
> - 依赖方向：上层依赖下层，禁止反向依赖
> - 核心业务层不依赖基础设施层（依赖倒置原则）

```
示例依赖方向（禁止逆向）：
  Presentation → Application → Domain → Infrastructure
```

### 2.2 跨模块通信

> [团队填写] 跨模块/服务通信方式，例如：
>
> - 同进程：通过接口（Interface）调用，禁止直接引用实现
> - 跨服务：REST / gRPC / 消息队列（MQ），选型原则是什么

### 2.3 禁止事项

> [团队填写] 明确架构层面的禁止行为，例如：
>
> - 禁止跨模块直接访问数据库
> - 禁止在 Repository 层写业务逻辑
> - 禁止循环依赖

---

## 3. API 设计原则

### 3.1 对外 API

> [团队填写] 例如：
>
> - RESTful 设计，遵循 HTTP 语义（GET/POST/PUT/DELETE）
> - 版本化：URL 路径版本（`/v1/`）
> - 响应格式统一：`{ "code": 0, "data": {}, "message": "" }`
> - 错误码体系：参见 `docs/standards/error-codes.md`

### 3.2 内部 API / 接口定义

> [团队填写] 例如：
>
> - 接口定义与实现分离，接口定义放在 `interfaces/` 目录
> - gRPC proto 文件统一放在 `proto/` 目录，版本管理由 team-standards 维护

### 3.3 API 变更原则

> [团队填写] 例如：
>
> - 已发布的 API 不得做 Breaking Change，使用新版本路径
> - 废弃字段保留至少 2 个版本周期

---

## 4. 数据模型原则

### 4.1 数据库选型原则

> [团队填写] 例如：
>
> - 结构化业务数据：关系型数据库（PostgreSQL 优先）
> - 缓存：Redis
> - 全文检索：Elasticsearch
> - 新服务选型须经架构评审

### 4.2 数据建模约定

> [团队填写] 例如：
>
> - 所有表必须有 `created_at`、`updated_at`、`deleted_at`（软删除）
> - 主键使用 UUID，不使用自增 ID（分布式场景）
> - 禁止在数据库层做业务逻辑（存储过程、触发器）

### 4.3 数据迁移

> [团队填写] 例如：
>
> - 所有 Schema 变更通过迁移脚本管理（Flyway / golang-migrate）
> - 迁移脚本只能追加，不能修改已执行的脚本
> - 生产环境迁移需 DBA Review

---

## 5. 扩展性与可观测性

### 5.1 扩展性设计

> [团队填写] 例如：
>
> - 无状态服务设计，Session 数据不存本地
> - 配置与代码分离，通过环境变量或配置中心注入
> - 耗时操作异步化，使用消息队列解耦

### 5.2 可观测性要求

> [团队填写] 三要素：Metrics / Tracing / Logging
>
> - Metrics：暴露 `/metrics` 端点，接入 Prometheus
> - Tracing：所有跨服务调用携带 Trace ID（OpenTelemetry）
> - Logging：结构化日志，必须包含 trace_id、service、level、message

---

## 6. 架构决策记录（ADR）

> 重大架构决策必须以 ADR（Architecture Decision Record）形式记录：
>
> - 存放路径：`docs/adr/NNNN-<title>.md`
> - 模板：参见 `docs/adr/0000-template.md`
> - 触发条件：技术选型、架构风格变更、跨团队影响的设计决策
>
> ADR 一旦通过 Review 并合并，视为团队共识，修改须提新 ADR。
