# go-clean-template

Go 分层后端脚手架范本。本文件是产品域术语表，只定义概念，不记录实现细节；后端实现术语见 `backend/CONTEXT.md`。随项目演进在此登记业务术语；当前内置的账号与认证术语描述随模板携带的示例模块。

## Language

### 账号与认证

**用户（User）**:
平台的登录主体，以邮箱全局唯一标识。
_Avoid_: 账号、账户、会员、member、account

**密码哈希（Password Hash）**:
用户密码经单向变换后的存储形态；原始密码任何环节不落库。
_Avoid_: 密文、加密密码

**access token**:
短有效期（分钟级）的自包含 JWT，客户端每次请求携带，用于证明用户身份；不可吊销，过期即失效。

**refresh token**:
长有效期（周级）的随机凭据，仅用于换取新令牌对；服务端只存其哈希，登出即吊销。
_Avoid_: 刷新令牌、长效 token

**登出（Logout）**:
吊销 refresh token 的行为；access token 仍在自然有效期内，至多容忍一个 access 有效期。

（示例模块保留说明见 README；替换为自有业务时同步增删本节术语。）
