# AuthN / AuthZ 统一方案：CLI + 远程 MCP + API Gateway

> **状态**：草案 · 待讨论  
> **日期**：2026-04-20  
> **作者**：基于 `spec/api-token-investigation.md` 演进  
> **目标读者**：架构师、后端工程师、安全团队

---

## 目录

1. [背景与目标](#1-背景与目标)
2. [核心约束](#2-核心约束)
3. [整体架构](#3-整体架构)
4. [协议选型与依据](#4-协议选型与依据)
5. [详细认证流程](#5-详细认证流程)
   - 5.1 [CLI 交互登录（Device Flow）](#51-cli-交互登录device-flow)
   - 5.2 [CLI 自动化（PAT）](#52-cli-自动化pat)
   - 5.3 [MCP 交互用户（OAuth 2.1 + PKCE）](#53-mcp-交互用户oauth-21--pkce)
   - 5.4 [MCP 程序化 Agent（PAT / Client Credentials）](#54-mcp-程序化-agentpat--client-credentials)
   - 5.5 [Token Exchange：MCP Server → API Gateway](#55-token-exchangemcp-server--api-gateway)
6. [各组件规格](#6-各组件规格)
   - 6.1 [IdP（Keycloak）](#61-idpkeycloak)
   - 6.2 [API Gateway（APISIX / Kong）](#62-api-gatewayapisix--kong)
   - 6.3 [MCP Server](#63-mcp-server)
   - 6.4 [CLI（cora）](#64-clicora)
7. [Token 格式与 Claims 规范](#7-token-格式与-claims-规范)
8. [Scope 设计](#8-scope-设计)
9. [安全考量](#9-安全考量)
10. [分阶段落地路线图](#10-分阶段落地路线图)
11. [主流社区参考实践](#11-主流社区参考实践)
12. [待讨论与开放问题](#12-待讨论与开放问题)
13. [参考规范与链接](#13-参考规范与链接)

---

## 1. 背景与目标

### 1.1 场景描述

系统由以下层次组成：

```
接入层：CLI（cora）  +  远程 HTTP MCP Server
            ↓                  ↓
        API Gateway（统一入口，AuthN + AuthZ 执行点）
            ↓
        后端业务服务（GitCode、openEuler portal 等）
```

**两类接入方**：

| 接入方 | 典型场景 |
|--------|---------|
| CLI（cora） | 开发者本地终端操作；CI/CD 自动化脚本 |
| 远程 MCP Server | Claude Desktop 等 AI 客户端的交互用户；程序化 AI Agent |

### 1.2 核心目标

1. **AuthN 统一**：无论通过 CLI 还是 MCP，用户身份由同一个 IdP 颁发和验证
2. **AuthZ 统一**：API Gateway 是唯一的权限执行点，CLI 和 MCP 路径的权限语义完全一致
3. **不重造轮子**：所有协议选用已发布的 IETF RFC 或广泛采用的业界标准
4. **零后端感知**：后端服务不解析 Token，只信任 Gateway 注入的 Header
5. **可审计**：每条请求都能还原到具体用户身份，无论经过几跳

---

## 2. 核心约束

### 2.1 MCP 规范禁止 Token 透传

MCP 规范（[draft 2025-03-26](https://modelcontextprotocol.io/specification/draft/basic/authorization)）明确规定：

> *"If the MCP server makes requests to upstream APIs, it may act as an OAuth client to them. The access token used at the upstream API is **a separate token**, issued by the upstream authorization server. The MCP server **MUST NOT** pass through the token it received from the MCP client."*

**含义**：

- MCP Client → MCP Server 的 Token：audience 绑定到 MCP Server
- MCP Server → API Gateway 的 Token：必须是**另一个独立 Token**，audience 绑定到 API Gateway
- 禁止将 MCP Client Token 直接转发给 API Gateway（"token passthrough" 被明确列为安全违规）

这是整个架构中最关键的约束，直接决定了 Token Exchange 是必选项。

### 2.2 MCP Server 对 OAuth 2.1 的强制要求

MCP 规范要求：

- 远程 HTTP MCP Server **必须**实现 [OAuth 2.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-13) Resource Server 角色
- **必须**实现 [RFC 9728](https://www.rfc-editor.org/rfc/rfc9728) Protected Resource Metadata
- **必须**支持 `Authorization: Bearer <token>` 接入
- **必须**验证 Token 的 `aud`（audience）claim 等于自身
- PKCE 使用 **S256**，禁止 plain 方法

### 2.3 Token audience 隔离

```
用户 Token（aud=mcp-server）  ≠  Gateway Token（aud=api-gateway）
```

两者来自同一 IdP，但 audience 不同，不可互换使用。

---

## 3. 整体架构

### 3.1 架构全景图

```
┌─────────────────────────────────────────────────────────────────────┐
│                    IdP（Keycloak ≥ 26.2）                            │
│                                                                       │
│  ┌──────────────┐  ┌───────────────┐  ┌──────────────────────────┐  │
│  │ Device Flow  │  │ Auth Code +   │  │  Token Exchange          │  │
│  │ RFC 8628     │  │ PKCE          │  │  RFC 8693                │  │
│  │ (CLI login)  │  │ (MCP OAuth)   │  │  (MCP Server → Gateway)  │  │
│  └──────────────┘  └───────────────┘  └──────────────────────────┘  │
│                                                                       │
│  JWKS 端点  ·  RFC 8414 AS Metadata  ·  OIDC Discovery              │
└───────────────────┬─────────────────────────────┬────────────────────┘
                    │                             │
        ┌───────────┘                             └──────────┐
        │                                                    │
┌───────▼───────────────────┐              ┌────────────────▼──────────────┐
│         CLI 路径           │              │          MCP 路径              │
├───────────────────────────┤              ├───────────────────────────────┤
│ cora auth login           │              │   远程 HTTP MCP Server         │
│   → Device Flow           │              │   OAuth 2.1 Resource Server   │
│   → 存储 JWT/PAT           │              │   RFC 9728 元数据发现          │
│                           │              │   Token Exchange Client        │
│ CI/CD 自动化               │              │                               │
│   → PAT（长期 token）      │              │ 交互用户                        │
│   → env var 注入           │              │   → Auth Code + PKCE          │
│                           │              │   → aud=mcp-server            │
│                           │              │                               │
│                           │              │ 程序化 Agent                    │
│                           │              │   → PAT / Client Credentials  │
│                           │              │   → aud=mcp-server            │
└───────────┬───────────────┘              └───────────────┬───────────────┘
            │ JWT                                          │ Token Exchange
            │ aud=api-gateway                              │ sub=user-id
            │ sub=user-id                                  │ aud=api-gateway
            └──────────────────────┬───────────────────────┘
                                   ▼
                   ┌───────────────────────────────┐
                   │         API Gateway            │
                   │  （APISIX / Kong）              │
                   │                               │
                   │  1. JWT 验签（JWKS）            │
                   │  2. aud=api-gateway 校验       │
                   │  3. Scope / RBAC AuthZ         │
                   │  4. 注入 X-User-Id header      │
                   │  5. 移除原始 Authorization     │
                   └───────────────┬───────────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
               GitCode API    Portal API    其他后端服务
               （只信任        （只信任       （只信任
               X-User-Id）    X-User-Id）    X-User-Id）
```

### 3.2 路径对比

| 维度 | CLI 交互 | CLI 自动化 | MCP 交互用户 | MCP 程序化 |
|------|---------|-----------|------------|----------|
| 获取凭据方式 | Device Flow | PAT 预生成 | Auth Code + PKCE | PAT / Client Creds |
| 凭据格式 | JWT（短期） | Opaque PAT | JWT（短期） | JWT / Opaque PAT |
| 到 Gateway 的 Token | 直接用户 JWT | PAT（Gateway 验证） | Token Exchange 换出的 JWT | Token Exchange 换出的 JWT |
| Gateway 看到的 aud | `api-gateway` | `api-gateway` | `api-gateway` | `api-gateway` |
| Gateway 看到的 sub | user-id | user-id | user-id（保留） | user-id（保留） |
| AuthZ 执行点 | Gateway | Gateway | Gateway | Gateway |

---

## 4. 协议选型与依据

| 场景 | 选用协议 | 选择理由 |
|------|---------|---------|
| CLI 登录 | **OAuth 2.0 Device Authorization Grant** ([RFC 8628](https://www.rfc-editor.org/rfc/rfc8628)) | 无浏览器重定向，终端友好；GitHub CLI / gcloud / AWS CLI 均采用此方案 |
| CLI 自动化凭据 | **PAT（Personal Access Token）** | 最简单的机器可读凭据，操作可审计；GitHub / GitLab / Stripe 均支持 |
| MCP 交互用户 | **OAuth 2.1 + Auth Code + PKCE** ([OAuth 2.1 draft](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-13) + [RFC 7636](https://www.rfc-editor.org/rfc/rfc7636)) | MCP 规范强制要求；PKCE 防止授权码截获，S256 算法 |
| MCP 程序化 Agent | **PAT** 或 **Client Credentials** ([RFC 6749 §4.4](https://www.rfc-editor.org/rfc/rfc6749#section-4.4)) | 无用户参与；PAT 更易调试，Client Credentials 更符合 OAuth 语义 |
| MCP Server → Gateway | **Token Exchange** ([RFC 8693](https://www.rfc-editor.org/rfc/rfc8693)) | MCP 规范禁止透传；Token Exchange 是唯一既保留 user sub 又重绑 audience 的标准协议 |
| Gateway Token 验签 | **JWKS + JWT** ([RFC 7517](https://www.rfc-editor.org/rfc/rfc7517) + [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519)) | 本地验签无状态，高性能；无需 Introspection 回调 |
| Token 吊销高安全场景 | **Token Introspection** ([RFC 7662](https://www.rfc-editor.org/rfc/rfc7662)) | 可选增强，金融/政务场景按需启用 |
| MCP 元数据发现 | **Protected Resource Metadata** ([RFC 9728](https://www.rfc-editor.org/rfc/rfc9728)) | MCP 规范强制；客户端通过 401 + WWW-Authenticate 自动发现 |
| AS 元数据发现 | **OAuth AS Metadata** ([RFC 8414](https://www.rfc-editor.org/rfc/rfc8414)) + **OIDC Discovery** | 自动获取 token/JWKS 端点，零硬编码 |
| Token 短期化 + 刷新 | **Refresh Token Rotation** | 短期 JWT（15 min）+ 长期 Refresh Token（可吊销） |
| 服务间身份 | **SPIFFE/SPIRE**（可选，后期引入） | K8s 场景下 mTLS 服务身份标准 |

---

## 5. 详细认证流程

### 5.1 CLI 交互登录（Device Flow）

**协议**：[RFC 8628 — OAuth 2.0 Device Authorization Grant](https://www.rfc-editor.org/rfc/rfc8628)

```
用户             cora CLI              IdP                  浏览器
 │                  │                   │                     │
 │  cora auth login │                   │                     │
 │─────────────────>│                   │                     │
 │                  │  POST /device/code│                     │
 │                  │  client_id=cora   │                     │
 │                  │  scope=openid ... │                     │
 │                  │──────────────────>│                     │
 │                  │  device_code      │                     │
 │                  │  user_code=XXXX   │                     │
 │                  │  verification_uri │                     │
 │                  │  expires_in=600   │                     │
 │                  │  interval=5       │                     │
 │                  │<──────────────────│                     │
 │  打开浏览器:      │                   │                     │
 │  https://idp/device               │                     │
 │  输入 user_code: XXXX             │                     │
 │<─────────────────│                   │                     │
 │                  │                   │   用户登录 + 确认     │
 │                  │                   │<────────────────────│
 │                  │  每 5s 轮询        │                     │
 │                  │  POST /token      │                     │
 │                  │  grant_type=      │                     │
 │                  │  urn:...device_code                     │
 │                  │  device_code=...  │                     │
 │                  │──────────────────>│                     │
 │                  │  (authorization_pending → 继续轮询)      │
 │                  │                   │                     │
 │                  │  access_token     │                     │
 │                  │  token_type=Bearer│                     │
 │                  │  expires_in=900   │                     │
 │                  │  refresh_token    │                     │
 │                  │<──────────────────│                     │
 │  ✓ 已登录        │                   │                     │
 │<─────────────────│                   │                     │
 │                  │ 存储: ~/.config/cora/token              │
```

**关键参数**：

```http
# Step 1：请求 device code
POST /realms/{realm}/protocol/openid-connect/auth/device
Content-Type: application/x-www-form-urlencoded

client_id=cora-cli&scope=openid%20offline_access%20api%3Aread%20api%3Awrite

# 响应
{
  "device_code": "GmRhmhcxhwAzkoEqiMEg_DnyEysNkuNhszIySk9eS",
  "user_code": "WDJB-MJHT",
  "verification_uri": "https://idp.example.com/device",
  "verification_uri_complete": "https://idp.example.com/device?user_code=WDJB-MJHT",
  "expires_in": 600,
  "interval": 5
}

# Step 2：轮询 token
POST /realms/{realm}/protocol/openid-connect/token
grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code
&device_code=GmRhmhcxhwAzkoEqiMEg_DnyEysNkuNhszIySk9eS
&client_id=cora-cli

# 轮询错误码
# authorization_pending → 继续等待
# slow_down            → interval += 5s
# access_denied        → 用户拒绝，终止
# expired_token        → 重新发起
```

**Token 存储优先级链**（参考 Google Workspace CLI 设计）：

```
1. 环境变量  CORA_TOKEN                    ← 最高优先级，CI/CD 场景
2. 环境变量  CORA_{SERVICE}_TOKEN          ← 服务级覆盖
3. ~/.config/cora/token.enc               ← AES-256-GCM 加密文件
4. ~/.config/cora/token.json             ← 明文（旧版兼容）
```

---

### 5.2 CLI 自动化（PAT）

**场景**：CI/CD pipeline、脚本、无交互环境

PAT 由用户在 IdP 管理界面或 `cora auth token create` 命令生成，本质是一个 audience 绑定到 `api-gateway` 的长期 Opaque Token 或 JWT。

```bash
# 生成 PAT（通过 cora CLI 或 IdP Web UI）
cora auth token create \
  --name "ci-pipeline" \
  --scope "api:read api:write" \
  --expires-in 90d

# 输出（仅显示一次）
CORA_TOKEN=cora_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxx

# CI/CD 使用
export CORA_TOKEN=cora_pat_xxx
cora gitcode issues list --owner openeuler --repo community
```

**PAT 直接调用 API Gateway**：

```http
GET /v1/gitcode/repos/openeuler/community/issues
Host: api-gateway.example.com
Authorization: Bearer cora_pat_xxxxxxxxxxx
```

Gateway 通过 Introspection（[RFC 7662](https://www.rfc-editor.org/rfc/rfc7662)）或直接 JWKS 验证 PAT，提取 `sub` 注入 `X-User-Id`。

---

### 5.3 MCP 交互用户（OAuth 2.1 + PKCE）

**协议**：[OAuth 2.1 draft](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-13) + [RFC 7636 PKCE](https://www.rfc-editor.org/rfc/rfc7636) + [RFC 9728](https://www.rfc-editor.org/rfc/rfc9728)

```
AI客户端           MCP Server              IdP               浏览器/用户
   │                   │                    │                    │
   │ MCP HTTP 请求     │                    │                    │
   │ (无 token)        │                    │                    │
   │──────────────────>│                    │                    │
   │                   │ HTTP 401           │                    │
   │                   │ WWW-Authenticate:  │                    │
   │                   │   Bearer           │                    │
   │                   │   resource_metadata│                    │
   │                   │   ="https://mcp.   │                    │
   │                   │   example.com/     │                    │
   │                   │   .well-known/...  │                    │
   │                   │   scope="api:read" │                    │
   │<──────────────────│                    │                    │
   │                   │                    │                    │
   │ GET /.well-known/oauth-protected-resource                   │
   │──────────────────>│                    │                    │
   │  { "authorization_servers":            │                    │
   │    ["https://idp.example.com"],        │                    │
   │    "scopes_supported": ["api:read",    │                    │
   │     "api:write"] }                     │                    │
   │<──────────────────│                    │                    │
   │                   │                    │                    │
   │ GET /.well-known/oauth-authorization-server                 │
   │───────────────────────────────────────>│                    │
   │  { token_endpoint, authorization_endpoint,                  │
   │    code_challenge_methods_supported: ["S256"] }             │
   │<───────────────────────────────────────│                    │
   │                   │                    │                    │
   │ 生成 PKCE 参数     │                    │                    │
   │ code_verifier = random(32 bytes)       │                    │
   │ code_challenge = BASE64URL(SHA256(code_verifier))           │
   │                   │                    │                    │
   │ 打开浏览器         │                    │                    │
   │ GET /authorize    │                    │                    │
   │   client_id=...   │                    │                    │
   │   response_type=code                   │                    │
   │   redirect_uri=http://localhost:PORT/cb│                    │
   │   scope=openid api:read                │                    │
   │   code_challenge=...                   │                    │
   │   code_challenge_method=S256           │                    │
   │   resource=https://mcp.example.com     │  ← RFC 8707       │
   │   state=<random>  │                    │                    │
   │───────────────────────────────────────────────────────────>│
   │                   │                    │ 用户登录 + 授权      │
   │                   │                    │<───────────────────│
   │                   │                    │                    │
   │ GET /callback?code=AUTH_CODE&state=... │                    │
   │<───────────────────────────────────────────────────────────│
   │                   │                    │                    │
   │ POST /token       │                    │                    │
   │   grant_type=authorization_code        │                    │
   │   code=AUTH_CODE  │                    │                    │
   │   redirect_uri=...│                    │                    │
   │   code_verifier=..│                    │                    │
   │   client_id=...   │                    │                    │
   │   resource=https://mcp.example.com     │                    │
   │───────────────────────────────────────>│                    │
   │  {access_token, token_type: "Bearer",  │                    │
   │   expires_in: 900, refresh_token}      │                    │
   │<───────────────────────────────────────│                    │
   │                   │                    │                    │
   │ MCP 请求          │                    │                    │
   │ Authorization:    │                    │                    │
   │   Bearer <token>  │                    │                    │
   │   (aud=mcp-server)│                    │                    │
   │──────────────────>│                    │                    │
   │                   │ 验证 token         │                    │
   │                   │ aud=mcp-server ✓   │                    │
   │                   │ 执行 Token Exchange│                    │
   │                   │ (见 5.5 节)        │                    │
```

**MCP Server 必须实现的发现端点**：

```
# RFC 9728 Protected Resource Metadata
GET https://mcp.example.com/.well-known/oauth-protected-resource

响应示例：
{
  "resource": "https://mcp.example.com",
  "authorization_servers": [
    "https://idp.example.com/realms/cora"
  ],
  "scopes_supported": ["api:read", "api:write", "api:admin"],
  "bearer_methods_supported": ["header"],
  "resource_documentation": "https://docs.example.com/mcp-api"
}
```

---

### 5.4 MCP 程序化 Agent（PAT / Client Credentials）

**两种方式**：

**方式 A：PAT（推荐，简单易调试）**

```bash
# 用户在管理界面生成绑定 mcp-server 的 PAT
CORA_MCP_TOKEN=cora_pat_xxxxxxxxxxxxxxx

# Agent 配置（如 Claude API 中的 MCP server 配置）
{
  "mcpServers": {
    "cora": {
      "url": "https://mcp.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${CORA_MCP_TOKEN}"
      }
    }
  }
}
```

**方式 B：Client Credentials（[RFC 6749 §4.4](https://www.rfc-editor.org/rfc/rfc6749#section-4.4)，适合机构级 Agent）**

```http
POST /realms/cora/protocol/openid-connect/token

grant_type=client_credentials
&client_id=my-agent-app
&client_secret=xxxxxx
&scope=api:read
&resource=https://mcp.example.com
```

响应中的 `access_token`（audience=mcp-server，sub=client-id）作为后续 MCP 请求的 Bearer token。MCP Server 仍需做 Token Exchange 换取 Gateway token，sub 保留 client-id 以便审计。

---

### 5.5 Token Exchange：MCP Server → API Gateway

**协议**：[RFC 8693 — OAuth 2.0 Token Exchange](https://www.rfc-editor.org/rfc/rfc8693)

这是整个方案的核心跳转，每次 MCP tool call 触发时，MCP Server 将用户 token 换成 Gateway 专用 token：

```
MCP Server                    IdP                    API Gateway
     │                          │                         │
     │ POST /token               │                         │
     │ Authorization: Basic      │                         │
     │   <client_id:secret>      │                         │
     │ grant_type=               │                         │
     │  urn:ietf:params:oauth:   │                         │
     │  grant-type:token-exchange│                         │
     │ subject_token=<user_token>│                         │
     │ subject_token_type=       │                         │
     │  urn:...:access_token     │                         │
     │ requested_token_type=     │                         │
     │  urn:...:access_token     │                         │
     │ resource=                 │                         │
     │  https://api-gw.example.com  ← RFC 8707            │
     │ scope=api:read            │  ← 按需 downscope       │
     │─────────────────────────>│                         │
     │                          │                         │
     │  {                        │                         │
     │   "access_token": "eyJ..",│                         │
     │   "issued_token_type":    │                         │
     │    "urn:...:access_token",│                         │
     │   "token_type": "Bearer", │                         │
     │   "expires_in": 300       │                         │
     │  }                        │                         │
     │<─────────────────────────│                         │
     │                          │                         │
     │ GET /v1/gitcode/issues    │                         │
     │ Authorization:            │                         │
     │   Bearer <exchanged_token>│                         │
     │   (aud=api-gateway        │                         │
     │    sub=original-user-id)  │                         │
     │─────────────────────────────────────────────────>  │
     │                          │  验签 JWKS              │
     │                          │  aud=api-gateway ✓      │
     │                          │  sub=user-id (AuthZ)    │
     │                          │  注入 X-User-Id          │
     │  响应                     │                         │
     │<─────────────────────────────────────────────────  │
```

**Keycloak 26.2 配置（[官方文档](https://www.keycloak.org/securing-apps/token-exchange)）**：

```
1. 在 Admin Console 中，为 mcp-server client 开启 "Standard token exchange" 开关
2. mcp-server 必须是 confidential client（有 client_secret）
3. 目标 client（api-gateway resource）需要允许来自 mcp-server 的 token exchange 请求
4. subject_token 的 aud claim 必须包含 mcp-server client id
```

**Token Exchange 的 downscoping**：

MCP Server 在 exchange 时可以请求比用户原始 scope 更小的权限集，实现最小权限原则：

```
用户原始 scope：api:read api:write api:admin
MCP tool 调用只需要：api:read
→ Token Exchange 请求中 scope=api:read
→ 换出的 token 只包含 api:read
```

**性能优化：Token Exchange 结果缓存**

Token Exchange 结果（exchanged token）应在 MCP Server 侧按 `(user_id, scope_set)` 缓存，有效期内复用，避免每次 tool call 都触发 Exchange：

```
cache_key = hash(subject_token + requested_scope)
cache_ttl = min(exchanged_token.expires_in - 30s, 270s)  # 留 30s 缓冲
```

---

## 6. 各组件规格

### 6.1 IdP（Keycloak）

**推荐版本**：Keycloak ≥ 26.2（[Standard Token Exchange 正式支持](https://www.keycloak.org/2025/05/standard-token-exchange-kc-26-2)）

**必须配置的 Realm 特性**：

| 配置项 | 值 | 说明 |
|--------|-----|------|
| `code_challenge_methods_supported` | `["S256"]` | MCP PKCE 强制要求 |
| Token Exchange | 开启 | Keycloak 26.2 默认开启，按 client 启用 |
| Access Token 有效期 | 900s（15分钟） | 短期 JWT，降低泄露风险 |
| Refresh Token 有效期 | 7天（交互用户）/ 90天（PAT） | — |
| Refresh Token 轮换 | 开启 | 每次使用刷新，旧 token 立即失效 |

**必须配置的 Client**：

| Client ID | 类型 | 说明 |
|-----------|------|------|
| `cora-cli` | Public | CLI 使用 Device Flow，无 secret |
| `cora-mcp-server` | Confidential | MCP Server，有 client_secret，用于 Token Exchange |
| `cora-api-gateway` | Resource | API Gateway 对应的 OAuth Resource |

**必须暴露的端点**：

```
# AS Metadata（RFC 8414）
GET https://idp.example.com/realms/cora/.well-known/oauth-authorization-server

# OIDC Discovery
GET https://idp.example.com/realms/cora/.well-known/openid-configuration

# JWKS
GET https://idp.example.com/realms/cora/protocol/openid-connect/certs

# Device Authorization
POST https://idp.example.com/realms/cora/protocol/openid-connect/auth/device

# Token
POST https://idp.example.com/realms/cora/protocol/openid-connect/token
```

---

### 6.2 API Gateway（APISIX / Kong）

**推荐**：[Apache APISIX](https://apisix.apache.org/)（[jwt-auth 插件](https://apisix.apache.org/docs/apisix/plugins/jwt-auth/) + [openid-connect 插件](https://apisix.apache.org/docs/apisix/3.10/plugins/openid-connect/)）

**Gateway 职责**：

1. **JWT 验签**：从 JWKS 端点自动获取公钥，本地验签，无需回调 IdP
2. **Audience 校验**：`aud` claim 必须包含 `api-gateway` 的 client URI
3. **AuthZ 执行**：根据 `scope` claim 和路由规则判断是否放行
4. **Header 注入**：提取 claims，注入标准 Header 给后端
5. **原始 Authorization Header 移除**：后端不得收到原始 token

**APISIX 路由配置示例（OIDC 插件）**：

```yaml
# 路由：GitCode 服务
uri: /v1/gitcode/*
plugins:
  openid-connect:
    client_id: cora-api-gateway
    client_secret: ${GATEWAY_CLIENT_SECRET}
    discovery: https://idp.example.com/realms/cora/.well-known/openid-configuration
    bearer_only: true          # 只接受 Bearer token，不做重定向
    realm: cora
    token_signing_alg_values_expected: RS256
    set_userinfo_header: false
    set_access_token_header: false
    # 将 sub claim 注入 upstream header
    userinfo_header_name: X-User-Info
  # 自定义 header 注入（通过 serverless 插件或 Lua 插件）
  # X-User-Id: <sub claim>
  # X-User-Scopes: <scope claim>
  proxy-rewrite:
    headers:
      remove:
        - Authorization        # 移除原始 token，后端不可见
```

**Kong 路由配置示例（[JWT 插件](https://developer.konghq.com/plugins/jwt/)）**：

```yaml
plugins:
  - name: jwt
    config:
      key_claim_name: iss
      claims_to_verify:
        - exp
        - nbf
      # JWKS 自动轮换
      jwks_uri: https://idp.example.com/realms/cora/protocol/openid-connect/certs
  - name: request-transformer
    config:
      remove:
        headers:
          - Authorization
      add:
        headers:
          - "X-User-Id:$(jwt.sub)"
          - "X-User-Scopes:$(jwt.scope)"
```

**Gateway 向后端注入的标准 Header**：

| Header | 来源 claim | 示例值 |
|--------|-----------|--------|
| `X-User-Id` | `sub` | `user:692e8a5d` |
| `X-User-Scopes` | `scope` | `api:read api:write` |
| `X-User-Email` | `email`（可选） | `user@example.com` |
| `X-Auth-Source` | 自定义（`cli` / `mcp`） | `mcp` |
| `X-Token-Iss` | `iss` | `https://idp.example.com/realms/cora` |

> **注意：Header 命名无 RFC 标准，为业界惯例**
>
> `X-User-Id` 等 Header 没有对应的 IETF RFC，各主流产品命名不同：
>
> | 产品 | 注入的 Header |
> |------|-------------|
> | AWS ALB + OIDC | `X-Amzn-Oidc-Identity`、`X-Amzn-Oidc-Data` |
> | APISIX openid-connect 插件 | 可配置，内置 `X-Userinfo` |
> | Kong JWT + request-transformer | 自定义配置 |
> | Envoy ext_authz | CheckResponse 自定义注入 |
>
> 模式一致，主流开源 Gateway 均原生支持，基本**零开发量**，仅需配置。
> 本项目统一使用上表命名，后端 middleware 按此读取。

---

### 6.3 MCP Server

**MCP 规范合规要求**（[Authorization spec](https://modelcontextprotocol.io/specification/draft/basic/authorization)）：

#### 必须实现

**1. Protected Resource Metadata（[RFC 9728](https://www.rfc-editor.org/rfc/rfc9728)）**

```
GET https://mcp.example.com/.well-known/oauth-protected-resource

{
  "resource": "https://mcp.example.com",
  "authorization_servers": [
    "https://idp.example.com/realms/cora"
  ],
  "scopes_supported": ["api:read", "api:write"],
  "bearer_methods_supported": ["header"],
  "resource_documentation": "https://docs.example.com/mcp"
}
```

**2. 401 响应携带 WWW-Authenticate**

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer
  resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource",
  scope="api:read"
```

**3. Token 验证**

```go
// 伪代码：MCP Server 收到请求后的验证流程
func validateToken(bearerToken string) (*Claims, error) {
    // 1. 从 JWKS 获取公钥（自动缓存和轮换）
    keySet := fetchJWKS("https://idp.example.com/realms/cora/protocol/openid-connect/certs")

    // 2. 验签
    token, err := jwt.Parse(bearerToken, keySet)

    // 3. 验证 audience 必须是 MCP Server 自身
    if !token.Audiences().Contains("https://mcp.example.com") {
        return nil, ErrInvalidAudience
    }

    // 4. 验证过期时间
    if token.Expiration().Before(time.Now()) {
        return nil, ErrTokenExpired
    }

    return token.Claims(), nil
}
```

**4. Token Exchange 调用（每次 tool call 前）**

```go
func exchangeToken(userToken string, requiredScope string) (string, error) {
    // 先查本地缓存
    cacheKey := hash(userToken + requiredScope)
    if cached, ok := tokenCache.Get(cacheKey); ok {
        return cached, nil
    }

    // 调用 IdP Token Exchange
    resp, err := http.PostForm(tokenEndpoint, url.Values{
        "grant_type":            {"urn:ietf:params:oauth:grant-type:token-exchange"},
        "subject_token":         {userToken},
        "subject_token_type":    {"urn:ietf:params:oauth:token-type:access_token"},
        "requested_token_type":  {"urn:ietf:params:oauth:token-type:access_token"},
        "resource":              {"https://api-gateway.example.com"},
        "scope":                 {requiredScope},
        "client_id":             {mcpClientID},
        "client_secret":         {mcpClientSecret},
    })

    // 缓存结果（留 30s 缓冲）
    tokenCache.Set(cacheKey, resp.AccessToken, resp.ExpiresIn-30)
    return resp.AccessToken, nil
}
```

#### 应该实现

- **Scope Step-up**：当 tool 需要更高权限时，返回 `403 + WWW-Authenticate: Bearer error="insufficient_scope" scope="api:admin"`
- **Refresh Token 处理**：当用户 token 过期时，尝试用 Refresh Token 刷新，刷新失败时返回 `401` 触发重新授权
- **审计日志**：记录每次 tool call 的 `user_id`、`tool_name`、`timestamp`、`exchanged_token_jti`

---

### 6.4 CLI（cora）

**凭据管理设计**（参考 [Google Workspace CLI 实现](https://github.com/googleworkspace/cli)）：

```
凭据加载优先级（高 → 低）：

1. CORA_TOKEN                    环境变量，直接使用
2. CORA_{SERVICE}_TOKEN          服务级环境变量（如 CORA_GITCODE_TOKEN）
3. ~/.config/cora/token.enc      AES-256-GCM 加密文件
4. ~/.config/cora/token.json     明文文件（旧版兼容）
```

**必须实现的命令**：

```bash
# 交互式登录（Device Flow）
cora auth login [--service gitcode]

# 查看认证状态
cora auth status
# 输出示例：
# gitcode   ✓  已认证 (expires in 12m, user: tommylike)
# portal    ✗  未配置

# 注销
cora auth logout [--service gitcode]

# PAT 管理
cora auth token create --name "ci" --scope "api:read" --expires-in 90d
cora auth token list
cora auth token revoke <token-id>
```

**API 调用时的 Token 刷新逻辑**：

```
发起请求
  → 检查 token 是否在 5 分钟内过期
  → 是：先用 Refresh Token 换新 Access Token
  → 否：直接发起请求
  → 收到 401：强制刷新，失败则提示 cora auth login
```

---

## 7. Token 格式与 Claims 规范

### 7.1 用户 JWT（CLI → Gateway 直接使用）

```json
{
  "iss": "https://idp.example.com/realms/cora",
  "sub": "user:692e8a5d-7156-f746-d25f-b5a2",
  "aud": ["cora-api-gateway"],
  "exp": 1776591403,
  "iat": 1776590503,
  "jti": "unique-token-id",
  "scope": "api:read api:write",
  "email": "user@example.com",
  "preferred_username": "tommylike",
  "client_id": "cora-cli"
}
```

### 7.2 MCP Client → MCP Server Token

```json
{
  "iss": "https://idp.example.com/realms/cora",
  "sub": "user:692e8a5d-7156-f746-d25f-b5a2",
  "aud": ["https://mcp.example.com"],           ← audience 绑定到 MCP Server
  "exp": 1776591403,
  "iat": 1776590503,
  "scope": "api:read api:write",
  "client_id": "claude-desktop"
}
```

### 7.3 Token Exchange 后（MCP Server → Gateway）

```json
{
  "iss": "https://idp.example.com/realms/cora",
  "sub": "user:692e8a5d-7156-f746-d25f-b5a2",   ← sub 不变，用户身份保留
  "aud": ["cora-api-gateway"],                    ← audience 重绑到 Gateway
  "exp": 1776590803,                              ← 较短有效期（5分钟）
  "iat": 1776590503,
  "scope": "api:read",                            ← 可能被 downscope
  "act": {                                        ← RFC 8693 actor claim
    "sub": "cora-mcp-server",                     ← 表示 MCP Server 代表用户操作
    "client_id": "cora-mcp-server"
  }
}
```

`act` claim 使 Gateway 审计日志能区分：这是 MCP Server 代表用户发出的请求，而非用户直接调用。

---

## 8. Scope 设计

### 8.1 Scope 命名规范

采用 `resource:action` 格式，与 GitHub / Google API 风格一致：

| Scope | 含义 |
|-------|------|
| `api:read` | 所有服务的只读操作 |
| `api:write` | 所有服务的写操作 |
| `api:admin` | 管理操作（创建 token、管理成员等） |
| `gitcode:read` | 仅 GitCode 服务只读（细粒度，可选） |
| `gitcode:write` | 仅 GitCode 服务写（细粒度，可选） |
| `openid` | OIDC 身份 |
| `offline_access` | 请求 Refresh Token |

### 8.2 默认 Scope 策略

- CLI 登录默认请求：`openid offline_access api:read api:write`
- MCP 初始化默认请求：`openid api:read`（最小权限，step-up 按需升级）
- PAT 创建时用户自选 scope

---

## 9. 安全考量

### 9.1 Token 安全

| 威胁 | 缓解措施 |
|------|---------|
| Token 泄露 | Access Token 有效期 15 分钟；Refresh Token 一次性使用（轮换）|
| Token 重放 | JWT `jti` claim + Gateway 侧短期 jti 黑名单（可选）|
| Token 混用 | `aud` claim 严格校验；MCP Server 拒绝 aud≠self 的 token |
| Token 透传 | MCP Server 禁止透传，Token Exchange 换出新 token |
| Confused Deputy | MCP Server 必须使用自身 credentials 发起 Exchange，不能盲传用户 token |

### 9.2 PKCE 安全

- 强制 `S256`，禁止 `plain` method（[OAuth 2.1 §4.1.1](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-13#section-4.1.1)）
- `code_verifier` 随机 32 字节，`state` 随机 16 字节防 CSRF
- Redirect URI 必须精确匹配预注册列表

### 9.3 MCP Server 凭据保护

- `client_secret` 通过 Vault / K8s Secret 注入，禁止硬编码
- Token Exchange 结果缓存在内存中，不落盘
- MCP Server 自身的 client_secret 泄露 = 所有用户 session 受影响 → 必须走 secret rotation

### 9.4 Admin Key / PAT 管理

- PAT 存储时使用 SHA-256 哈希，原值仅在创建时显示一次
- PAT 支持 scope 限制和过期时间
- PAT 泄露时可单独吊销，不影响其他 token

### 9.5 API Gateway 安全

Gateway 是整个系统的信任边界，后端服务收到的 `X-User-Id` 等 Header 本身无法自证来源合法性——任何能直连后端的调用方都可以伪造这些 Header。因此**网络层和传输层的防护是必选项**，而非可选建议。

#### 9.5.1 核心原则

- 后端服务**禁止**直接暴露在公网，防火墙只允许来自 Gateway 的入站流量
- 后端必须**主动剥离**客户端携带的同名 Header，防止 Header 注入攻击：
  ```go
  // 后端 middleware：拒绝外部伪造的 X-User-* Header
  // Gateway 在转发前已移除原始 Authorization，后端收到的 X-User-* 均来自 Gateway
  // 若请求未经过 Gateway（如内网直连），则这些 Header 不应存在
  ```

#### 9.5.2 Gateway → 后端安全方案对比

| 方案 | 机制 | 适用场景 | 开发工作量 |
|------|------|---------|-----------|
| **网络隔离** | 防火墙 / 安全组限制后端只接受 Gateway IP | 基础保障，必须实施 | 零 |
| **mTLS（推荐）** | Gateway 持有客户端证书，后端验证证书 | 需更强安全保障 | 配置为主 |
| **服务网格（K8s）** | Istio / Linkerd 自动注入 mTLS，无需改业务代码 | K8s 部署环境 | 零业务代码改动 |
| **内部签名 JWT** | Gateway 用私钥签发内部 JWT 传给后端，后端验签 | 无服务网格但需可验证身份 | 后端引入 JWT 验签库 |

#### 9.5.3 推荐落地策略

**K8s 环境**：优先采用 Istio/Linkerd 服务网格，mTLS 自动化，业务代码零改动，同时解决 Gateway→后端和服务间的双向身份验证。SPIFFE/SPIRE 作为底层工作负载身份标准可在后期引入。

**非 K8s 环境**：网络隔离（必选）+ 内部签名 JWT（推荐）：

```
# Gateway 签发内部 JWT（有效期极短，如 30s）
{
  "iss": "api-gateway",
  "sub": "<original user sub>",
  "scope": "api:read",
  "exp": <now + 30s>,
  "iat": <now>
}
# 后端用 Gateway 公钥验签，Header 内容可自证合法性
# 不再依赖纯网络隔离
```

#### 9.5.4 Header 防伪造措施（所有方案均需）

无论采用哪种传输安全方案，后端应用层都需要：

1. **剥离入站的 `X-User-*` Header**：在 middleware 最前端清除，防止客户端伪造
2. **只读取 Gateway 注入的值**：在剥离后再由 Gateway（或内部 JWT 解析）重新写入
3. **拒绝无身份标识的请求**：`X-User-Id` 为空时直接返回 `401`

---

## 10. 分阶段落地路线图

### 阶段 0（1-2 天）：IdP 部署

- 部署 Keycloak ≥ 26.2
- 配置 Realm：cora
- 创建 Clients：cora-cli（public）、cora-mcp-server（confidential）、cora-api-gateway（resource）
- 验证 JWKS 端点和 Device Flow 可用

**验收标准**：`cora auth login` 完成 Device Flow，获取 JWT

---

### 阶段 1（3-5 天）：CLI AuthN

- 实现 `cora auth login`（Device Flow）
- 实现 `cora auth status`
- 实现 Token 优先级链（env var > 加密文件 > 明文文件）
- 实现 Token 自动刷新逻辑
- CLI 调用 API Gateway 使用 Bearer JWT

**验收标准**：
```bash
cora auth login                          # 完成 Device Flow
export CORA_TOKEN=xxx && cora gitcode issues list  # env var 覆盖
cora auth status                         # 显示有效期和用户名
```

---

### 阶段 2（1 周）：API Gateway 配置

- 部署 APISIX / Kong
- 配置 JWT 验签插件（JWKS 自动轮换）
- 配置 aud claim 校验
- 配置 Header 注入（X-User-Id 等）
- 配置路由和 Scope-based AuthZ

**验收标准**：CLI 发出的 JWT 经 Gateway 验证，后端收到正确的 `X-User-Id` Header

---

### 阶段 3（1 周）：MCP Server AuthN/AuthZ

- 实现 `/.well-known/oauth-protected-resource`（RFC 9728）
- 实现 Bearer Token 接收和验证（aud=mcp-server）
- 实现 Token Exchange 调用（RFC 8693）
- 实现 Token Exchange 结果缓存
- 支持交互用户（OAuth 2.1 + PKCE 流程由 MCP 客户端完成）
- 支持程序化 Agent（PAT 传入 Authorization header）

**验收标准**：
```
Claude Desktop → MCP Server → Token Exchange → API Gateway → GitCode API
整个链路返回正确数据，Gateway 审计日志显示正确的 user sub 和 act claim
```

---

### 阶段 4（按需）：增强安全

- PAT 管理界面（创建/吊销/查看）
- Scope Step-up Authorization（MCP tool 按需升级权限）
- 统一退出（OIDC Back-Channel Logout）
- CLI 凭据 AES-256-GCM 加密（参考 Google Workspace CLI）
- Token Introspection（高安全场景替代 JWKS 本地验签）
- SPIFFE/SPIRE 服务间 mTLS（K8s 场景）

---

## 11. 主流社区参考实践

| 社区/工具 | CLI 认证 | API/MCP 认证 | 借鉴点 |
|-----------|---------|------------|--------|
| **[GitHub CLI](https://cli.github.com/)** | Device Flow (RFC 8628) | OAuth App token / PAT / GitHub App JWT | Device Flow 实现参考；PAT 分类存储（type: oauth vs PAT） |
| **[gcloud CLI](https://cloud.google.com/sdk/docs/authorizing)** | Browser OAuth + Device Flow | ADC 优先级链（env→文件→元数据服务→ADC） | 凭据优先级链设计；`auth status` 诊断命令 |
| **[Stripe CLI](https://docs.stripe.com/cli/api_keys)** | API Key 直接配置 | Restricted API Keys（细粒度 scope） | PAT scope 设计；restricted key 限权模式 |
| **[kubectl](https://kubernetes.io/docs/reference/access-authn-authz/authentication/)** | kubeconfig Bearer / OIDC | ServiceAccount token（aud 绑定） | Token audience 隔离；kubeconfig 优先级链 |
| **[AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sso.html)** | SSO + Device Flow | IAM Role 临时 credentials + STS Token Exchange | Token Exchange 概念（STS AssumeRoleWithWebIdentity） |
| **[Cloudflare Workers MCP](https://developers.cloudflare.com/agents/guides/remote-mcp-server/)** | — | OAuth 2.1 + PKCE，remote MCP | MCP OAuth 实现参考；Worker 侧 Token 验证 |

---

## 12. 待讨论与开放问题

以下问题需要在方案评审中确认：

### 12.1 IdP 选型确认

- 是否选用 Keycloak？替代方案：Ory Hydra+Kratos（云原生）、Casdoor（国内友好）、Auth0（商业 SaaS）
- Keycloak 26.2 的 Token Exchange 是否满足 RFC 8693 完整语义（含 `act` claim）？

### 12.2 Token Exchange 缓存粒度

- 缓存 key 是否用 `hash(user_token)` 还是 `hash(user_id + scope)`？
- 前者更精准但 token 轮换时缓存失效；后者更稳定但需要额外解析 token

### 12.3 PAT 的实现位置

- PAT 由 IdP（Keycloak）管理，还是业务层单独实现一套 PAT 系统？
- Keycloak 的 [Service Account + Client Credentials](https://www.keycloak.org/docs/latest/server_admin/#_service_accounts) 是否可以作为 PAT 的底层实现？

### 12.4 MCP 程序化 Agent 的 user sub

- Client Credentials 流程没有真实用户，Gateway 看到的 `sub` 是 `client_id`
- 后端审计日志如何区分"用户操作"和"Agent 代操作"？
- 是否需要在 MCP 层强制要求程序化 Agent 也必须绑定一个用户身份（防止以机器身份绕过 per-user 限额）？

### 12.5 Gateway 选型确认

- APISIX vs Kong vs Envoy/Istio？
- 是否已有现成部署的 Gateway？

### 12.6 Token Exchange 的性能影响

- 每次 MCP tool call 都需要 Token Exchange（如果缓存失效），增加约 20-50ms 延迟
- 是否可接受？是否需要 MCP Server 与 IdP 之间走内网专线？

### 12.7 多租户场景

- 是否需要支持多租户（不同组织有不同 Keycloak Realm）？
- MCP Server 如何在不同 Realm 间路由 Token Exchange 请求？

---

## 13. 参考规范与链接

### IETF RFC

| RFC | 标题 | 用途 |
|-----|------|------|
| [RFC 6749](https://www.rfc-editor.org/rfc/rfc6749) | OAuth 2.0 Authorization Framework | 基础框架 |
| [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) | Bearer Token Usage | Bearer 携带方式 |
| [RFC 7519](https://www.rfc-editor.org/rfc/rfc7519) | JSON Web Token (JWT) | Token 格式 |
| [RFC 7517](https://www.rfc-editor.org/rfc/rfc7517) | JSON Web Key (JWK) | 公钥格式 |
| [RFC 7636](https://www.rfc-editor.org/rfc/rfc7636) | PKCE | 授权码安全增强 |
| [RFC 7662](https://www.rfc-editor.org/rfc/rfc7662) | Token Introspection | 可吊销 Token 验证 |
| [RFC 8414](https://www.rfc-editor.org/rfc/rfc8414) | OAuth AS Metadata | 自动发现 |
| [RFC 8628](https://www.rfc-editor.org/rfc/rfc8628) | Device Authorization Grant | CLI 登录 |
| [RFC 8693](https://www.rfc-editor.org/rfc/rfc8693) | Token Exchange | MCP→Gateway 换 Token |
| [RFC 8707](https://www.rfc-editor.org/rfc/rfc8707) | Resource Indicators | Token audience 绑定 |
| [RFC 9068](https://www.rfc-editor.org/rfc/rfc9068) | JWT Profile for OAuth Access Tokens | JWT claim 规范 |
| [RFC 9728](https://www.rfc-editor.org/rfc/rfc9728) | OAuth Protected Resource Metadata | MCP 发现机制 |
| [OAuth 2.1 draft](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-13) | OAuth 2.1 | MCP 强制要求 |

### MCP 规范

| 文档 | 链接 |
|------|------|
| MCP Authorization Spec（draft） | https://modelcontextprotocol.io/specification/draft/basic/authorization |
| MCP Security Best Practices | https://modelcontextprotocol.io/specification/draft/basic/security_best_practices |
| MCP Auth Extensions | https://github.com/modelcontextprotocol/ext-auth |

### 组件文档

| 组件 | 文档链接 |
|------|---------|
| Keycloak Token Exchange | https://www.keycloak.org/securing-apps/token-exchange |
| Keycloak 26.2 Token Exchange GA | https://www.keycloak.org/2025/05/standard-token-exchange-kc-26-2 |
| APISIX JWT Auth Plugin | https://apisix.apache.org/docs/apisix/plugins/jwt-auth/ |
| APISIX OpenID Connect Plugin | https://apisix.apache.org/docs/apisix/3.10/plugins/openid-connect/ |
| Kong JWT Plugin | https://developer.konghq.com/plugins/jwt/ |
| GitHub CLI OAuth Library (Go) | https://github.com/cli/oauth |
| Google Workspace CLI（凭据设计参考） | https://github.com/googleworkspace/cli |

### 延伸阅读

| 文章 | 链接 |
|------|------|
| MCP OAuth 2.1 实战指南 | https://www.scalekit.com/blog/implement-oauth-for-mcp-servers |
| Remote MCP in the Real World | https://medium.com/@yagmur.sahin/remote-mcp-in-the-real-world-oauth-2-1-9d149de6e475 |
| Spring AI MCP + OAuth2 | https://spring.io/blog/2025/04/02/mcp-server-oauth2/ |
| Token Exchange Keycloak 实战 | https://dev.to/iamdevbox/keycloak-token-exchange-a-step-by-step-guide-to-oauth-20-token-exchange-4eia |

---

## 14. 关键缩写词表

| 缩写 | 全称 | 释义 |
|------|------|------|
| **AuthN** | Authentication | 认证：验证"你是谁"，确认主体身份 |
| **AuthZ** | Authorization | 授权：验证"你能做什么"，控制资源访问权限 |
| **MCP** | Model Context Protocol | 模型上下文协议，Anthropic 定义的 AI 模型与外部工具/服务之间的标准通信协议 |
| **IdP** | Identity Provider | 身份提供方：负责用户认证、颁发 Token 的中央服务（如 Keycloak） |
| **SP** | Service Provider | 服务提供方：依赖 IdP 验证用户身份的业务服务 |
| **JWT** | JSON Web Token | 一种紧凑、URL 安全的 Token 格式，包含 Header、Payload、Signature 三段，可本地验签（[RFC 7519](https://www.rfc-editor.org/rfc/rfc7519)） |
| **JWK** | JSON Web Key | 以 JSON 表示的加密密钥（[RFC 7517](https://www.rfc-editor.org/rfc/rfc7517)） |
| **JWKS** | JSON Web Key Set | JWK 的集合，通常由 IdP 暴露为公开端点供各方获取验签公钥 |
| **PKCE** | Proof Key for Code Exchange | 授权码交换证明：防止授权码截获攻击，通过 code_verifier / code_challenge 配对验证（[RFC 7636](https://www.rfc-editor.org/rfc/rfc7636)） |
| **PAT** | Personal Access Token | 个人访问令牌：用户手动生成的长期凭据，常用于自动化、CI/CD 场景 |
| **SSO** | Single Sign-On | 单点登录：用户一次登录，即可访问多个关联系统 |
| **OIDC** | OpenID Connect | 基于 OAuth 2.0 的身份层协议，通过 ID Token 传递用户身份信息 |
| **OAuth** | Open Authorization | 开放授权框架：允许第三方应用在用户授权下安全访问资源（[RFC 6749](https://www.rfc-editor.org/rfc/rfc6749)） |
| **RBAC** | Role-Based Access Control | 基于角色的访问控制：通过角色分配权限，而非直接给用户授权 |
| **ABAC** | Attribute-Based Access Control | 基于属性的访问控制：根据用户、资源、环境的属性组合动态决策权限 |
| **PDP** | Policy Decision Point | 策略决策点：集中计算"是否允许访问"的服务（如 OPA） |
| **PEP** | Policy Enforcement Point | 策略执行点：实际拦截请求并向 PDP 查询决策结果的组件（如 API Gateway） |
| **SAML** | Security Assertion Markup Language | 基于 XML 的企业级身份联邦协议，常用于传统 SSO 场景 |
| **mTLS** | Mutual TLS | 双向 TLS：客户端和服务端互相验证对方证书，常用于服务间身份验证 |
| **SPIFFE** | Secure Production Identity Framework For Everyone | 云原生服务身份标准，为每个工作负载分配可验证的身份（SVID） |
| **SPIRE** | SPIFFE Runtime Environment | SPIFFE 的参考实现，负责颁发和轮换 SVID |
| **SVID** | SPIFFE Verifiable Identity Document | SPIFFE 颁发的身份文档，通常以 X.509 证书或 JWT 形式呈现 |
| **ADC** | Application Default Credentials | Google Cloud 的凭据自动发现机制，按优先级链查找可用凭据 |
| **STS** | Security Token Service | 安全令牌服务：负责颁发、续期、交换临时安全凭据（AWS 术语，对应 RFC 8693） |
| **Scope** | — | OAuth 权限范围声明，用于限定 Token 可访问的资源和操作（如 `api:read`） |
| **Audience（aud）** | — | JWT claim，声明该 Token 的预期接收方；接收方必须校验 aud 与自身匹配 |
| **Subject（sub）** | — | JWT claim，标识 Token 所代表的主体（通常为用户 ID 或 client ID） |
| **Issuer（iss）** | — | JWT claim，标识颁发该 Token 的 IdP 地址 |
| **Actor（act）** | — | JWT claim（RFC 8693 扩展），标识代表主体（sub）执行操作的中间方（如 MCP Server） |
| **Introspection** | Token Introspection | 通过向 IdP 查询验证 Opaque Token 有效性的机制（[RFC 7662](https://www.rfc-editor.org/rfc/rfc7662)） |
| **Downscoping** | — | Token Exchange 时请求比原始 Token 更小权限集的操作，实现最小权限原则 |
| **Device Flow** | Device Authorization Grant | 适用于无浏览器环境的 OAuth 流程，用户在另一台设备上完成授权（[RFC 8628](https://www.rfc-editor.org/rfc/rfc8628)） |
| **Client Credentials** | Client Credentials Grant | 机器对机器的 OAuth 流程，以 client_id + client_secret 换取 Token，无用户参与（[RFC 6749 §4.4](https://www.rfc-editor.org/rfc/rfc6749#section-4.4)） |
| **Token Exchange** | OAuth 2.0 Token Exchange | 将一种 Token 换成另一种 Token 的 OAuth 扩展机制，可重绑 audience、downscope 权限（[RFC 8693](https://www.rfc-editor.org/rfc/rfc8693)） |
| **Resource Indicator** | — | 在 OAuth 请求中显式声明目标资源 URI，使 Token audience 绑定到特定服务（[RFC 8707](https://www.rfc-editor.org/rfc/rfc8707)） |
| **Step-up Auth** | Step-up Authorization | 当现有 Token 权限不足时，触发增量授权流程获取更高权限的机制 |
| **Back-Channel Logout** | OIDC Back-Channel Logout | IdP 主动通知各 SP 用户已退出，实现真正的统一登出 |
| **OPA** | Open Policy Agent | 开源通用策略引擎，使用 Rego 语言定义访问控制策略（CNCF 毕业项目） |
| **APISIX** | Apache APISIX | 高性能云原生 API Gateway，支持丰富的认证鉴权插件 |
| **CLI** | Command-Line Interface | 命令行界面，本文中特指 `cora` CLI 工具 |
| **CI/CD** | Continuous Integration / Continuous Delivery | 持续集成 / 持续交付，自动化构建、测试、部署流水线 |

---

*文档版本：v0.1 草案 · 2026-04-20 · 待架构评审*
