# 创建数据库迁移文件

请为以下需求创建数据库迁移脚本：$ARGUMENTS

## 执行步骤

1. 理解需求（如果描述不清楚，先列出理解和假设再动手）
2. 检查 `migrations/` 目录中现有的迁移文件，了解命名规则和最新序号
3. 生成迁移文件

## 迁移文件规范

参照 `docs/standards/build.md` 中的数据库迁移约定：

### 文件命名

```
migrations/<timestamp>_<description>.sql
# 示例：migrations/20250301143000_add_user_phone_index.sql
```

timestamp 格式：`YYYYMMDDHHmmss`（当前时间）

### 文件结构

每个迁移文件必须同时包含 up 和 down：

```sql
-- migrate:up
<正向迁移 SQL>

-- migrate:down
<回滚 SQL>
```

### SQL 编写约定

- 所有表名、列名使用 snake_case
- 新表必须包含：`id`（UUID）、`created_at`、`updated_at`、`deleted_at`（软删除）
- 不添加 FOREIGN KEY 约束（应用层维护关系）
- 索引命名：`idx_<table>_<columns>`
- 唯一索引命名：`uniq_<table>_<columns>`
- 大表加索引使用 `CREATE INDEX CONCURRENTLY`（避免锁表）

### 安全检查

在迁移中涉及以下操作时，**必须提示我确认**后再继续：

- DROP TABLE / DROP COLUMN（数据会丢失）
- 修改现有列类型（可能失败或数据截断）
- 大表全表更新（可能锁表）

## 完成后

生成迁移文件后，提示我运行以下命令验证：

```bash
make migrate-status  # 查看待执行迁移
make migrate         # 执行迁移
```
