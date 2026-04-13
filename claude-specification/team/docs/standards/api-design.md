# API 设计规范

> 本文件定义团队 HTTP/gRPC API 的设计约定。
> 所有对外和对内的接口设计均应遵循本规范，确保服务间协作一致性。

---

## RESTful 设计原则

### URL 设计

```
# 格式
/<version>/<resource>/<id>/<sub-resource>

# 示例
GET    /v1/users              # 获取用户列表
POST   /v1/users              # 创建用户
GET    /v1/users/123          # 获取单个用户
PUT    /v1/users/123          # 全量更新用户
PATCH  /v1/users/123          # 部分更新用户
DELETE /v1/users/123          # 删除用户
GET    /v1/users/123/orders   # 获取用户的订单列表
```

**规则：**

- 资源名使用**复数名词**（`users` 而非 `user`），kebab-case
- URL 中不包含动词（用 HTTP 方法表达语义）
- 嵌套不超过 2 层（`/users/123/orders`，不要 `/users/123/orders/456/items`）
- 非 CRUD 操作使用动词后缀：`POST /v1/orders/123/cancel`

### HTTP 方法语义

| 方法 | 语义 | 幂等 | 安全 |
| --- | --- | --- | --- |
| GET | 查询，不改变状态 | 是 | 是 |
| POST | 创建资源 / 触发操作 | 否 | 否 |
| PUT | 全量替换资源 | 是 | 否 |
| PATCH | 部分更新资源 | 否 | 否 |
| DELETE | 删除资源 | 是 | 否 |

### 版本策略

> [团队填写] 选择版本方案：
>
> - **URL Path 版本**（推荐）：`/v1/users`，简单直观
> - **Header 版本**：`Accept: application/vnd.api+json; version=1`

**版本升级规则：**

- PATCH 版本：Bug 修复，无需升级 API 版本
- 新增可选字段：不升级版本
- 删除字段 / 修改字段类型 / 变更行为：必须升级 Major 版本

---

## 请求与响应格式

### 统一响应结构

**成功响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "abc-123",
    "name": "example"
  }
}
```

**列表响应（含分页）：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [...],
    "pagination": {
      "total": 100,
      "page": 1,
      "page_size": 20,
      "has_next": true
    }
  }
}
```

**错误响应：**

```json
{
  "code": 10001,
  "message": "user not found",
  "details": [
    {
      "field": "user_id",
      "reason": "no user with id 'abc-123'"
    }
  ]
}
```

### HTTP 状态码使用

| 状态码 | 含义 | 使用场景 |
| --- | --- | --- |
| 200 | OK | GET/PUT/PATCH/DELETE 成功 |
| 201 | Created | POST 创建成功 |
| 204 | No Content | DELETE 成功（无返回体） |
| 400 | Bad Request | 请求参数校验失败 |
| 401 | Unauthorized | 未认证（Token 缺失或过期） |
| 403 | Forbidden | 已认证但无权限 |
| 404 | Not Found | 资源不存在 |
| 409 | Conflict | 资源冲突（重复创建、版本冲突） |
| 422 | Unprocessable Entity | 参数格式正确但业务校验失败 |
| 429 | Too Many Requests | 限流 |
| 500 | Internal Server Error | 服务端未预期错误 |

---

## 错误码体系

### 编号规则

```
<服务编号><模块编号><序号>

示例：
10001 - 用户服务(1) + 用户模块(00) + 第1个错误
20101 - 订单服务(2) + 支付模块(01) + 第1个错误
```

> [团队填写] 各服务编号分配表：
>
> | 编号 | 服务 |
> | --- | --- |
> | 1 | [填写] |
> | 2 | [填写] |

### 通用错误码

| 错误码 | 含义 |
| --- | --- |
| 0 | 成功 |
| 400xx | 通用参数错误 |
| 401xx | 认证错误 |
| 403xx | 权限错误 |
| 404xx | 资源不存在 |
| 429xx | 限流 |
| 500xx | 服务内部错误 |

---

## 分页规范

### 偏移量分页（适合管理后台）

```
GET /v1/users?page=2&page_size=20

# 默认值
page_size: 20（最大 100）
page: 1
```

### 游标分页（适合移动端 / 信息流）

```
GET /v1/users?cursor=eyJpZCI6MTIzfQ&limit=20

# 响应中返回 next_cursor
{
  "data": {
    "items": [...],
    "next_cursor": "eyJpZCI6MTQzfQ",
    "has_next": true
  }
}
```

> [团队填写] 默认分页方式：偏移量 / 游标

---

## 查询与过滤

```
# 等值过滤
GET /v1/users?status=active&role=admin

# 范围过滤
GET /v1/orders?created_at_gte=2024-01-01&created_at_lte=2024-12-31

# 排序（默认降序）
GET /v1/users?sort_by=created_at&sort_order=desc

# 模糊搜索
GET /v1/users?q=john

# 字段选择（可选）
GET /v1/users?fields=id,name,email
```

**命名约定：**

- 过滤参数使用 snake_case
- 范围过滤后缀：`_gte`（>=）、`_lte`（<=）、`_gt`（>）、`_lt`（<）
- 布尔参数使用 `is_` 前缀：`is_active=true`

---

## 接口兼容性

### 不兼容变更（必须升级版本）

- 删除已有字段
- 修改字段类型（如 `string` → `int`）
- 修改字段语义（如 `status` 枚举值含义变化）
- 修改 URL 路径
- 新增必填参数

### 兼容变更（无需升级版本）

- 新增可选请求参数
- 新增响应字段
- 新增新的 API endpoint
- 新增错误码

---

## gRPC 规范

> [团队填写] 如团队使用 gRPC，补充以下约定：

### Proto 文件组织

```
proto/
├── <service>/
│   └── v1/
│       ├── <service>.proto     # Service 定义
│       └── <resource>.proto    # Message 定义
└── common/
    └── v1/
        └── pagination.proto    # 公共消息定义
```

### 命名约定

- Package：`<company>.<service>.v1`
- Service：`PascalCase`，以 `Service` 结尾
- RPC 方法：`PascalCase`，动词开头（`GetUser`、`ListOrders`、`CreateOrder`）
- Message：`PascalCase`，Request/Response 后缀
- 字段：`snake_case`

---

## API 文档要求

- HTTP API 使用 OpenAPI 3.0 规范描述，文件为 `openapi.yaml`
- gRPC API 的文档写在 Proto 文件注释中
- 每个 endpoint 必须包含：描述、请求参数说明、响应示例、错误码列表
- API 文档与代码同步更新，不允许文档落后于实现
