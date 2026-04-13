# 数据库设计与迁移规范

> 本文件定义团队数据库设计、命名和迁移操作的统一约定。
> Claude 在生成 Migration 或修改数据库相关代码时必须遵循本规范。

---

## 命名规范

### 表名

- 使用 **snake_case**，**复数形式**
- 关联表命名：`<表A>_<表B>`，按字母序排列
- 不使用数据库保留字

```
✅ users, orders, order_items, user_roles
❌ User, OrderItem, user_role, data, status
```

### 字段名

- 使用 **snake_case**
- 布尔字段使用 `is_` / `has_` / `can_` 前缀
- 时间字段使用 `_at` 后缀
- 外键使用 `<关联表单数>_id`

```
✅ user_id, is_active, created_at, has_permission
❌ userId, active, createTime, permission
```

### 索引命名

```
idx_<table>_<columns>        # 普通索引
udx_<table>_<columns>        # 唯一索引
fk_<table>_<ref_table>       # 外键约束

示例：
idx_orders_user_id
udx_users_email
idx_orders_status_created_at
```

---

## 主键策略

> [团队填写] 选择主键类型：
>
> - **UUID v4**：全局唯一，适合分布式系统，无序
> - **UUID v7**：时间有序 UUID，兼顾唯一性和索引性能（推荐）
> - **自增 ID**：简单高效，但分布式场景受限
> - **Snowflake**：有序长整型，需要额外基础设施

---

## 必备字段

每张业务表必须包含以下字段：

```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),  -- 或按团队主键策略
created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
deleted_at  TIMESTAMP NULL                               -- 软删除标记
```

**规则：**

- 所有时间字段存储为 **UTC**
- 使用**软删除**（`deleted_at IS NOT NULL` 表示已删除），不物理删除业务数据
- `updated_at` 通过数据库触发器或 ORM 自动维护

---

## 字段设计原则

### 类型选择

| 场景 | 推荐类型 | 避免 |
| --- | --- | --- |
| 主键 | UUID / BIGINT | INT（容量不足） |
| 金额 | DECIMAL(19,4) | FLOAT / DOUBLE |
| 状态枚举 | VARCHAR + 应用层约束 | 数据库 ENUM（变更困难） |
| JSON 数据 | JSONB（PostgreSQL） | TEXT 存 JSON |
| 长文本 | TEXT | VARCHAR(9999) |
| 布尔 | BOOLEAN | TINYINT |

### 约束要求

- NOT NULL：除非业务上确实允许空值，否则必须 NOT NULL
- DEFAULT：有合理默认值的字段必须设置 DEFAULT
- 唯一约束：业务上唯一的字段（email、手机号）必须加唯一索引
- 外键：[团队填写] 是否使用数据库外键约束（推荐应用层维护，避免数据库外键）

---

## 索引设计

### 何时加索引

- WHERE 条件中频繁使用的字段
- JOIN 关联字段
- ORDER BY / GROUP BY 字段
- 唯一性约束字段

### 索引原则

1. **单表索引不超过 5 个**（写入性能考虑）
2. **联合索引遵循最左前缀原则**，高选择性字段在前
3. **覆盖索引**优先于回表查询
4. **不要在低基数字段单独建索引**（如 `status` 只有 3 个值）
5. **大表加索引必须走 Migration + Online DDL**，不允许直接 ALTER

---

## Migration 规范

### 文件命名

```
migrations/<timestamp>_<description>.sql

示例：
migrations/20240315143000_create_users_table.sql
migrations/20240315150000_add_email_index_to_users.sql
```

- 时间戳格式：`YYYYMMDDHHmmss`
- 描述使用 snake_case，说明变更内容
- 每个 Migration 文件必须包含 **UP** 和 **DOWN** 部分

### Migration 文件结构

```sql
-- +migrate Up
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      VARCHAR(255) NOT NULL,
    name       VARCHAR(100) NOT NULL,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL
);

CREATE UNIQUE INDEX udx_users_email ON users (email) WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX IF EXISTS udx_users_email;
DROP TABLE IF EXISTS users;
```

### Migration 规则

1. **已部署的 Migration 文件禁止修改**——只能通过新 Migration 修正
2. **每个 Migration 只做一件事**（不要在一个文件中建表 + 改另一张表）
3. **必须可回滚**——DOWN 部分必须完整
4. **大表变更必须评估影响**：
   - 新增列：必须有 DEFAULT 值或允许 NULL
   - 删除列：先停止代码引用 → 部署 → 再删列（两步走）
   - 加索引：使用 `CONCURRENTLY`（PostgreSQL）避免锁表
5. **数据回填（Backfill）** 不放在 Migration 中，单独编写脚本
6. **涉及大表 DDL，提前知会 DBA**

### 数据回填规范

```sql
-- 文件：scripts/backfill_20240315_user_status.sql
-- 说明：将所有未设置 status 的用户默认设置为 active
-- 影响行数预估：约 50000 行
-- 是否可重复执行：是

UPDATE users
SET status = 'active', updated_at = NOW()
WHERE status IS NULL;
```

**要求：**

- 回填脚本必须**幂等**（可重复执行）
- 注明预估影响行数
- 大批量数据分批处理（每批 1000-5000 行）

---

## 查询规范

### 必须遵守的规则

1. **禁止 SELECT \***——明确列出需要的字段
2. **禁止无 WHERE 的 UPDATE / DELETE**
3. **分页查询必须有排序条件**——否则结果不确定
4. **大表查询必须走索引**——未命中索引的慢查询需优化
5. **多租户场景**：所有查询必须携带 `tenant_id` 过滤

### 慢查询标准

> [团队填写] 慢查询阈值：
>
> - 单次查询 > [填写] ms 视为慢查询
> - 慢查询必须记录日志并定期 Review

---

## Claude 行为约束

Claude 在执行数据库相关操作时必须遵守：

1. **生成 Migration 时**：必须包含 UP 和 DOWN，并遵循命名规范
2. **修改表结构前**：展示完整 SQL 并等待用户确认
3. **不执行 DROP TABLE / TRUNCATE**：除非用户明确要求并确认
4. **不生成 SELECT \***：必须明确列出字段
5. **涉及已上线表的变更**：提醒用户评估影响并考虑两步走策略
