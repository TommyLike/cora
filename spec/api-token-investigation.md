# 统一账号与 API Token 体系架构方案

> 面向多服务集成场景：通过一个身份/Token 贯穿所有服务，权限可控、可审计、可演进。

---

## 一、问题背景与核心诉求

在网站/平台开发中，常见需求是将多个服务（自研 + 开源）集成到同一账号体系下，并希望：

1. 用户侧：一次登录，所有服务通用（SSO）
2. API 侧：一个 Token，调用所有服务的 API
3. 权限侧：集中管理，各服务一致执行
4. 工程侧：对存量开源服务尽量零侵入

关键是理解这些诉求背后其实是三件不同的事，方案差别很大。

---

## 二、把"统一"拆开看：三个层次

| 层次          | 内容                 | 解决难度      | 工具/协议                          |
|-------------|--------------------|-----------|--------------------------------|
| 身份统一（AuthN） | 一次登录，所有服务知道"你是谁"   | 低，已成熟     | OAuth 2.0 / OIDC / SAML        |
| 授权统一（AuthZ） | 一次授权，所有服务知道"你能做什么" | 高，业务相关    | RBAC / ABAC / PDP (OPA, Cedar) |
| Token 统一    | 一张票据跨所有服务调用        | 中，取决于下游服务 | JWT / Token Exchange           |

> **误区**：很多团队把三件事当成一件事解决，结果架构很重、落地很慢。正确做法是**按层递进**。

---

## 三、协议与技术选型基础

### 3.1 认证协议

- **OAuth 2.0**：授权框架，解决"第三方代表用户调 API"的问题
- **OIDC（OpenID Connect）**：在 OAuth 2.0 上加身份层，ID Token 告诉你"用户是谁"
- **SAML**：更老更重，企业 SSO 场景
- **推荐**：现代 API-first 场景选 **OIDC + OAuth 2.0**

### 3.2 Token 格式

| 类型           | 优点          | 缺点                   | 适用场景        |
|--------------|-------------|----------------------|-------------|
| JWT（自包含）     | 本地验签快、无状态   | 难以吊销、信息可解码           | 高性能、短期有效    |
| Opaque Token | 可随时吊销、信息不泄漏 | 每次需 introspection 回调 | 金融/政务等高安全场景 |

**生产环境混合策略**：短期 JWT（15 分钟）+ 长期 Refresh Token（opaque，可吊销）。

### 3.3 关键 RFC

- **RFC 7662 Token Introspection**：Opaque Token 验证端点
- **RFC 8693 Token Exchange**：服务间 Token 换发，权限可 downscope
- **RFC 7519 JWT** + **RFC 7515 JWS**：Token 格式与签名

### 3.4 授权模型

- **Scope 模型**（OAuth 原生）：粗粒度，如 `read:orders`
- **RBAC**（基于角色）：企业 IAM 常用
- **ABAC / ReBAC**（基于属性/关系）：云原生趋势，OPA / Cedar / SpiceDB
- **PDP/PEP 分离**：决策点与执行点解耦，是授权架构核心思想

---

## 四、理想架构（适用于自研服务）

### 4.1 整体分层

```
      ┌──────────────┐
      │   客户端     │  Web / App / SDK
      └──────┬───────┘
             │ ② 携带 JWT
┌────────────┴────────────┐
│     API Gateway         │  Token 验证 · 鉴权拦截 · 路由 · 上下文注入
└──┬───────┬──────────┬───┘
   │③JWKS  │④策略查询  │⑤转发（注入 X-User-Id 等）
   ▼       ▼          ▼
┌──────┐ ┌──────┐  ┌──────────────────────┐
│ IdP  │ │ PDP  │  │ 用户/订单/支付/内容 服务 │
│OIDC  │ │ OPA  │  │ （只信任 Gateway Header）│
└──────┘ └──────┘  └──────────────────────┘
       ① 登录获取 Token（客户端 → IdP）
```

### 4.2 各层职责

**API Gateway**
- 本地验签 JWT（或调 IdP introspection）
- 调 PDP 做鉴权决策
- 抽取 Token claims，注入成可信 Header（`X-User-Id`、`X-User-Scopes`）
- 后端服务只信任 Gateway 注入的 Header，不再自己解析 Token —— **这是"统一"最关键的一步**

**身份提供方（IdP）**
- 用户登录 / MFA / 社交登录联邦
- 颁发 Access Token + Refresh Token
- 暴露 JWKS 端点供验签
- 暴露 introspection 端点供精确吊销检查

**策略决策点（PDP）**
- 独立服务，用 Rego / Cedar 等策略语言定义权限
- 接受 "principal + resource + action" 输入，返回 allow/deny
- 策略代码化、可版本管理、可单测

**后端服务**
- 不解析 Token、不做认证
- 信任 Gateway 注入的身份 Header
- 内部资源级鉴权时再调 PDP

### 4.3 关键设计决策

| 问题 | 推荐方案 |
|------|----------|
| Gateway 还是服务验签？ | **Gateway 统一验签并注入 Header**，服务保持无感 |
| 服务间调用身份传递？ | **Token Exchange (RFC 8693)** 或双令牌（用户 Token + 服务 Token） |
| API Key 和用户 Token 如何统一？ | Gateway 层归一化，后端只认 Header，感知不到调用方是人还是程序 |
| 服务身份怎么办？ | **SPIFFE/SPIRE** 做服务身份 + mTLS |

---

## 五、存量 OSS 服务的现实考量（以 Discourse 为例）

### 5.1 核心判断

> 存量开源服务（Discourse、GitLab、Nextcloud 等）**无法被零侵入地改造成接受你自己 IdP 颁发的 JWT 调 API**。它们有自己的用户体系和 API 密钥机制，强行改造意味着维护 fork、升级地狱。

但实现"用户感知到的账号统一"成本可以极低，而"一个 Token 调所有 API"则要妥协或走翻译路线。

### 5.2 Discourse 的实际认证能力

**作为 SP（把登录委托给外部 IdP）**
- **OpenID Connect 插件**（官方，推荐）—— 标准 OIDC
- **OAuth2 Basic 插件**（官方）—— 通用 OAuth2
- **SAML 插件**（官方）—— 企业场景
- **DiscourseConnect**（原生 SSO）—— HMAC-SHA256 签名的自有协议

**作为 IdP（Discourse 自己提供登录）**
- DiscourseConnect Provider 模式，一般不建议用

**API 认证方式（只有这两种）**
- **Admin API Key**：`Api-Key` + `Api-Username` 两个 Header，`Api-Username: system` 代表系统调用，指定用户名则代表该用户
- **User API Key**：通过 `User-Api-Key` Header 传，主要给移动 App 用

**Discourse 不能做的事**：原生接受外部 JWT 作为 API 鉴权（需要写插件）。

### 5.3 针对存量 OSS 的简化架构

```
          ┌──────────────┐
          │   中央 IdP   │  Keycloak / OIDC Provider
          └──────┬───────┘
                 │ 用户 SSO 登录
                 ▼
    ┌─────────────────────────────┐
    │      API Gateway            │
    │  - 验签外部 JWT              │
    │  - 凭据翻译（关键）          │
    │  - 每个 OSS 服务一个翻译插件 │
    └──┬─────────────────┬────────┘
       │ JWT → Api-Key    │ JWT → 原生 Header
       ▼                  ▼
   ┌─────────┐        ┌─────────┐
   │Discourse│        │ 其他 OSS│
   │自有鉴权  │        │ 自有鉴权 │
   └─────────┘        └─────────┘
```

**核心思路**：用户持有中央 JWT，Gateway 翻译成每个服务的原生认证方式再转发。OSS 服务保持原样。

### 5.4 凭据翻译的具体实现（Discourse 案例）

```
外部请求：Authorization: Bearer <中央JWT>
        ↓
Gateway 识别路径 /forum/* → 目标 Discourse
        ↓
验签 JWT，提取 external_id
        ↓
查映射：external_id → Discourse username
        ↓
重写 Header：
  Api-Key:      <系统级 Admin Key>
  Api-Username: <对应的 Discourse 用户名>
移除原 Authorization Header
        ↓
Discourse 看到合法的原生请求
```

实现方式可选：Kong 自定义 Lua 插件 / APISIX Lua 或 Wasm 插件 / Envoy ext_authz filter，核心代码几十行。

**映射表获取**：调 Discourse Admin API `GET /u/by-external/{external_id}.json` 拿到用户名，Gateway 缓存几秒钟即可。

**安全要点**：Admin API Key 权限极大，必须放在 Gateway 侧用 Vault / HSM 保护，绝不外泄。

### 5.5 权限和审计怎么处理

- **保留 OSS 服务自己的权限模型**，不要用 OPA 接管其内部权限
- 通过 OIDC claim 把中央 IdP 的组/角色映射到 Discourse groups，一次配置自动同步
- 审计：Gateway 翻译后 `Api-Username` 是真实用户，Discourse 原生日志准确
- Gateway 侧再记一份调用日志，形成双份审计

---

## 六、分阶段落地路线图

### 阶段 0（半天）：部署中央 IdP

- 推荐 **Keycloak**（开源、功能全、可迁移）
- 或已有的 OIDC Provider
- 配置用户目录、MFA、社交登录等

### 阶段 1（每服务半天到一天）：SSO 上线

- 为每个存量 OSS 服务启用 OIDC 插件
- 配置自动用户创建（auto-provisioning）
- 映射 `email`、`external_id`、groups 等字段
- **效果**：用户层面的"统一登录"达成

### 阶段 2（1–2 周，可选）：API Token 统一

- 引入 API Gateway（**APISIX** 或 **Kong**）
- 为每个 OSS 服务写凭据翻译插件
- 建立中央身份与各服务账号的映射机制
- **效果**：外部一个 JWT 可调所有服务 API

### 阶段 3（按需）：增强能力

- 统一登出（OIDC back-channel logout）
- 细粒度审计聚合
- 服务间 Token Exchange（仅自研服务）
- 密钥轮换自动化（Vault 集成）

> **重要**：很多团队其实只需要阶段 0+1，已能解决 80% 的"统一"体验问题。阶段 2 是硬成本投入，要看实际调用量和必要性。

---

## 七、产品选型建议清单

### 7.1 IdP（身份提供方）

| 产品 | 类型 | 推荐场景 |
|------|------|----------|
| Keycloak | 开源，功能全 | 大多数场景首选 |
| Ory Hydra + Kratos | 开源，云原生 | 高性能、云原生团队 |
| Casdoor | 开源，中文友好 | 国内团队、需要中文文档 |
| Auth0 / Okta | 商业 SaaS | 快速上线、可接受订阅费 |
| AWS Cognito / Azure AD B2C | 云厂商 | 已深度绑定某云 |

### 7.2 API Gateway

| 产品 | 类型 | 推荐场景 |
|------|------|----------|
| APISIX | 开源，Apache | 高性能、插件生态好 |
| Kong | 开源 + 商业 | 社区大、文档齐 |
| Envoy / Istio | 开源，云原生 | 已用 K8s / Service Mesh |
| AWS API Gateway / Azure APIM | 云商业 | 云厂商一站式 |

### 7.3 PDP（授权引擎，仅自研服务需要）

| 产品 | 类型 | 特点 |
|------|------|------|
| OPA (Open Policy Agent) | 开源，CNCF 毕业 | Rego 语言，通用策略引擎 |
| AWS Cedar | 开源 | 语法更易读，资源级授权 |
| SpiceDB | 开源 | Google Zanzibar 风格，关系型授权 |
| Casbin | 开源 | 轻量级，多语言 SDK |

### 7.4 推荐组合

**开源自建（推荐给中大型团队）**
```
Keycloak (IdP) + APISIX/Kong (Gateway) + OPA (PDP, 自研服务用)
```

**云原生服务网格**
```
Istio + 外部 OIDC Provider + SPIFFE 服务身份
```

**商业一站式**
```
Auth0/Okta + AWS API Gateway + Cedar
```

---

## 八、常见陷阱与对策

| 陷阱 | 后果 | 对策 |
|------|------|------|
| 每个服务自己解析 JWT | 权限不统一、配置散落 | Gateway 统一验签 + 注入 Header |
| 试图用 OPA 接管 OSS 内部权限 | 维护地狱、与 OSS 逻辑冲突 | OSS 保留自有权限模型，OPA 只管自研服务 |
| 直接让 OSS 服务接受 JWT | 需要写/维护插件 | Gateway 凭据翻译 |
| Admin API Key 分发给业务层 | 权限泄露风险 | 只放 Gateway，用 Vault 保护 |
| Service A 拿用户 Token 调 B | 无法区分代理还是直连 | Token Exchange 或双令牌 |
| 一次性上全量方案 | 周期长、落地风险高 | 按阶段 0→1→2→3 演进 |
| JWT 永不过期 | 无法吊销 | 短期 JWT + Refresh Token |

---

## 九、适用边界

**这套方案适合**
- Discourse、GitLab、Nextcloud、Grafana、Jenkins、Jupyter、Harbor、MinIO 等主流 OSS
- 以上都支持 OIDC/OAuth2 接入 + 自有 API 密钥机制，翻译模式普适

**不适合的场景**
- 对外开放给第三方开发者的 API（他们拿到你家的 JWT，翻译模式只适合 Gateway 内部）
- 需要跨服务原子事务授权的场景（凭据翻译做不了联动鉴权）
- 极端安全要求（Admin Key 集中在 Gateway 是风险集中点，需要 HSM 保护）

---

## 十、快速决策树

```
需要实现"账号统一"？
    │
    ├─ 只需要用户登录统一 → 阶段 0 + 阶段 1（OIDC 插件），成本最低
    │
    ├─ 需要 API Token 统一 →
    │   │
    │   ├─ 全是自研服务 → 完整架构：Gateway + IdP + PDP + JWT 到底
    │   │
    │   └─ 有存量 OSS 服务 → Gateway 凭据翻译模式（阶段 2）
    │
    └─ 需要跨服务精细授权 → 自研部分用 OPA，OSS 保留自有权限模型
```

---

## 附录：关键术语速查

| 术语     | 含义                                                |
|--------|---------------------------------------------------|
| SSO    | Single Sign-On，一次登录多处使用                           |
| IdP    | Identity Provider，身份提供方                           |
| SP     | Service Provider，服务提供方                            |
| AuthN  | Authentication，认证（你是谁）                            |
| AuthZ  | Authorization，授权（你能做什么）                           |
| PDP    | Policy Decision Point，策略决策点                       |
| PEP    | Policy Enforcement Point，策略执行点                    |
| JWT    | JSON Web Token                                    |
| JWKS   | JSON Web Key Set，公钥集合                             |
| OIDC   | OpenID Connect                                    |
| RBAC   | Role-Based Access Control                         |
| ABAC   | Attribute-Based Access Control                    |
| ReBAC  | Relationship-Based Access Control                 |
| mTLS   | Mutual TLS，双向 TLS 认证                              |
| SPIFFE | Secure Production Identity Framework For Everyone |

---

*文档整理自架构讨论，适用于 2026 年及之后的主流技术栈选型。*