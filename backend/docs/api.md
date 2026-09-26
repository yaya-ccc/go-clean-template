# 知岛后端接口文档

## 一、基础说明

- **服务地址**：`http://127.0.0.1:8080`（以部署环境为准）
- **业务接口前缀**：`/api`；健康检查 `/healthz` 独立
- **数据格式**：请求与响应均为 JSON（`Content-Type: application/json`）
- **认证方式**：登录后的大部分接口需在请求头携带 access token：

  ```
  Authorization: Bearer <access_token>
  ```

- **错误响应**：所有接口出错时统一返回：

  ```json
  { "code": "错误码", "message": "用户可读的中文提示" }
  ```

  参数校验失败统一返回 400 `invalid_input`，不携带字段级细节；具体原因仅供服务端日志排查。
  部分错误响应额外携带可选字段 `request_id`（与响应头 `X-Request-Id` 一致），示例：

  ```json
  {
    "code": "invalid_input",
    "message": "请求参数无效",
    "request_id": "c1c1c1c1c1c1c1c1"
  }
  ```

  | 错误码 | HTTP 状态码 | 含义 |
  |---|---|---|
  | `invalid_input` | 400 | 请求参数缺失或格式不合规 |
  | `bad_credentials` | 401 | 邮箱或密码错误 |
  | `unauthorized` | 401 | 未登录 / 登录已过期，应引导重新登录 |
  | `email_taken` | 409 | 邮箱已被注册 |
  | `internal_error` | 500 | 服务器内部错误 |
  | `database_error` | 500 | 数据库故障；文案与 internal_error 相同，仅 code 可区分归因 |
  | `service_unavailable` | 503 | 服务暂不可用（如健康检查探测数据库失败） |
  | `operation_timeout` | 503 | 请求处理超时；**不承诺操作未生效**，不要盲目重试注册等写操作 |
  | `route_not_found` | 404 | 请求的接口不存在 |
  | `method_not_allowed` | 405 | 请求方法不被该接口允许 |

  未列举的错误码一律按 `internal_error` 处理。

## 二、认证流程说明

系统采用「双令牌」机制：

1. **注册或登录**成功后，服务端返回 `access_token`（有效期 30 分钟），同时通过 `Set-Cookie` 下发 `refresh_token`（HttpOnly Cookie，有效期 30 天，前端 JS 不可读）。
2. 前端后续请求把 `access_token` 放在 `Authorization: Bearer` 头中访问需登录的接口。
3. `access_token` 过期后，接口返回 `401 unauthorized`；前端调用 **刷新接口**（浏览器自动携带 refresh cookie）换取新的 `access_token`，然后重试原请求。
4. refresh token 也过期（超过 30 天未使用）时，刷新接口同样返回 401，前端应跳转登录页。
5. **登出**时调用登出接口，服务端吊销 refresh 凭据并清除 cookie。

简化时序：

```
注册/登录 ──► 拿到 access_token(30分钟) + refresh cookie(30天)
    │
    ▼
携带 Bearer 访问业务接口 ──401──► 调用 /api/auth/refresh 换新 token ──► 重试
    │                                        │
    │                                   refresh 也失效
    ▼                                        ▼
登出：吊销 refresh，清 cookie            跳转登录页
```

## 三、接口明细

### 1. 注册

`POST /api/auth/register`

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| email | string | 是 | 邮箱格式；服务端去空白并转小写后作为账号标识 |
| password | string | 是 | 最短 8 字节、最长 72 字节（bcrypt 输入上限，超出返回 400） |
| nickname | string | 否 | 最长 64 字符；不传时默认取邮箱 @ 前部分 |

**请求示例**

```json
{
  "email": "tom@example.com",
  "password": "12345678",
  "nickname": "汤姆"
}
```

**成功响应 200**（同时 Set-Cookie 下发 refresh_token）

```json
{
  "id": 1,
  "email": "tom@example.com",
  "nickname": "汤姆",
  "created_at": "2026-09-23T10:00:00Z",
  "access_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**失败示例**

```json
{ "code": "email_taken", "message": "邮箱已被注册" }
```

### 2. 登录

`POST /api/auth/login`

**请求体**

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| email | string | 是 | 邮箱格式 |
| password | string | 是 | 无长度要求 |

**请求示例**

```json
{
  "email": "tom@example.com",
  "password": "12345678"
}
```

**成功响应 200**（结构同注册，同时 Set-Cookie 下发 refresh_token）

```json
{
  "id": 1,
  "email": "tom@example.com",
  "nickname": "汤姆",
  "created_at": "2026-09-23T10:00:00Z",
  "access_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**失败示例**

```json
{ "code": "bad_credentials", "message": "邮箱或密码错误" }
```

### 3. 刷新 access token

`POST /api/auth/refresh`

access token 过期后调用。**无需请求体**，浏览器自动携带 refresh cookie。

**成功响应 200**（同时下发新的 refresh cookie）

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**失败响应**

- 凭据缺失/无效/过期（`401 unauthorized`）：服务端会同时清除 refresh cookie，前端应跳转登录页。
- 服务端故障（`503` / `500`）：**cookie 不会被清除**，服务恢复后用户可直接重试刷新，不应被登出。

### 4. 登出

`POST /api/auth/logout`

吊销服务端 refresh 凭据并清除 cookie。幂等：未携带 cookie 或凭据本就不存在时均返回成功。

**成功响应 200**

```json
{
  "status": "ok"
}
```

**失败响应**（`503` / `500`）：服务端吊销失败，此时**不会清除 cookie**、不返回成功——
客户端应提示「登出未完成」而非「已安全退出」，用户可重试。

### 5. 获取当前用户信息

`GET /api/users/me`　——　**需认证**（`Authorization: Bearer <access_token>`）

**成功响应 200**

```json
{
  "id": 1,
  "email": "tom@example.com",
  "nickname": "汤姆",
  "created_at": "2026-09-23T10:00:00Z"
}
```

**失败示例**

```json
{ "code": "unauthorized", "message": "登录已过期，请重新登录" }
```

### 6. 健康检查

`GET /healthz`　——　供部署侧探活，无需认证

**成功响应 200**

```json
{
  "status": "ok"
}
```

**失败响应 503**（数据库不可用，`service_unavailable`）

```json
{ "code": "service_unavailable", "message": "服务暂不可用" }
```

## 四、调用示例（curl）

```bash
# 1. 注册（-c 保存 cookie 供刷新用）
curl -c jar.txt -X POST http://127.0.0.1:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"tom@example.com","password":"12345678"}'

# 2. 访问需登录接口
curl http://127.0.0.1:8080/api/users/me \
  -H "Authorization: Bearer <access_token>"

# 3. access token 过期后刷新（-b 携带 refresh cookie，-c 更新新 cookie）
curl -b jar.txt -c jar.txt -X POST http://127.0.0.1:8080/api/auth/refresh

# 4. 登出
curl -b jar.txt -c jar.txt -X POST http://127.0.0.1:8080/api/auth/logout
```
