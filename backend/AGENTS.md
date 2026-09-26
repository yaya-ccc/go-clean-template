# backend/AGENTS.md — 后端最终约束

本文件是后端的最终约束，面向在 `backend/` 下工作的开发人员与 AI 助手；后端实现术语见 `backend/CONTEXT.md`，产品域术语见根 `CONTEXT.md`。设计过程的历史决策记录（ADR）已删除，其结论已收拢进本文与相关注释。

## 目录骨架

```
backend/internal/
├── domain/                 # 领域层：实体、值对象、业务规则、仓储契约；按模块分包
│   └── user/               # User、TokenPair、NormalizeEmail、Repository 接口；fakes/ 共享测试假件
├── application/            # 应用层：用例编排；按模块分包
│   ├── auth/               # Service（用例入口）、TokenIssuer 抽象、dto/（AuthResult 等）
│   └── user/
├── infrastructure/
│   ├── acl/                # 外部服务防腐适配器（port 在业务侧，见「外部服务隔离」章节）
│   └── persistence/        # 自有数据技术（存储与缓存）的仓储实现；按技术分子包
│       └── mysql/          # 仓储实现 + po/（表形状）+ mapper.go（唯一转换点）+ errors.go（仓储边界分类）
├── presentation/
│   ├── api/                # 业务模块包：auth/ users/ 各含 handler.go（路由注册+handler+cookie 等协议细节）、dto/
│   ├── response/           # 错误翻译表 errTable + OK/Fail/Ack/InvalidBind 渲染
│   ├── middleware/         # ProcessLog、RequireAuth、ZapRecovery
│   └── router/             # 集中注册路由、决定鉴权分组；依赖由 main 注入
└── kernel/
    ├── apperr/             # Error/ErrCode/codes.go（错误码唯一注册表）
    ├── applog/             # 全局 logger + 过程日志 context kv
    └── config/             # config.yaml 加载
cmd/server/main.go          # 组装根：config → logger → mysql → repos → services → router
```

## 分层职责与依赖方向

依赖方向（import 检查口径，违反即结构性错误）：

- domain：只 import 标准库与 kernel。
- application：import domain 与 kernel；禁止 infrastructure、presentation、cmd。
- infrastructure：import domain 与 kernel；禁止 application、presentation、cmd。
- presentation：import application、domain（仅 DTO 视图组装与合码需要）、kernel；禁止 infrastructure、cmd。
- presentation 内部：业务模块包（api/ 下按模块分包）互不 import；协议公共件（router、response、middleware）不 import 业务模块包，鉴权所需的应用服务由 router 注入。
- kernel：零业务 import 依赖；可依赖具体技术选型（zap、go-yaml）。
- cmd：可以 import 一切，是唯一允许「具体实现接线」的位置。

「在哪写什么」：

- 业务概念/规则/不变量 → `domain/<模块>`：实体与值对象零标签（无 json/gorm tag），仓储契约是接口。
- 用例编排 → `application/<模块>`：Service 方法签名固定 `(ResultType, error)`；多于一个成功产出收拢进本模块 `dto/`（无 json tag、可持有领域对象）；单值或无产出不包装。协议细节（cookie、状态码）禁止出现在此层。
- 表访问/SQL/驱动错误翻译 → `infrastructure/persistence/mysql`：表形状只在 `po/`，po↔domain 转换只在 `mapper.go`；确实表达不了的查询允许仓储内 Raw SQL，但 SQL 不出仓储。
- HTTP 协议（路由、绑定、校验、渲染、cookie、鉴权）→ `presentation/api/<模块>`；新路由在 `router` 集中注册并决定鉴权分组。
- 依赖组装 → 只在 `cmd/server/main.go` 与 `router.New(Deps)`：仓储、应用服务都在 main 构造后注入；模块包与 router 只收现成依赖，nil 依赖视为组装错误直接返回错误。

## 错误体系

- 错误值一律 `apperr.ErrCode.With(op, ...)` 构造；任何非 apperr 错误在出现第一时间转换（出生点转换不变量）。
- 新错误码只在 `kernel/apperr/codes.go` 单点声明 + `presentation/response` 的 errTable 登记（一码一状态一文案），其余位置一律引用常量、不内联字符串；未登记码与非 apperr 错误在渲染时降级 `internal_error`，细节只进日志。
- 判断路径：`apperr.CodeIs(err, code)` 按最外层语义判断（外层行为决策）；`errors.Is(err, code)` 沿链匹配事实码（应用层分类仓储基础错误）。二者不可互换。
- 仓储 Find 类查询未命中不是错误：返回 `(nil, nil)`，由应用层判空产出内部事实码（如 `user_not_found`，不入公开翻译表）；写入唯一冲突由仓储边界分类直接产出业务码（如 `email_taken`）；未知失败归一 `database_error` 保留 cause。
- 防用户枚举的合码发生在使用它的 handler 内：登录将 `user_not_found` 就地合为 `bad_credentials`（并用固定 dummy bcrypt 拉平时间侧信道），refresh 将 `refresh_token_not_found` 合为 `unauthorized`；handler 忘记合码时未登记码自动降级 500，失败方向是报错而非暴露。

## 校验与 DTO

- 请求体校验全部在表现层 DTO 的 `Validate() error`（统一 400 `invalid_input`，无字段级细节）；应用层信任入参。显式例外：登录语境坏邮箱放行进查询自然失败，归一化用 `domain/user.NormalizeEmail`，保持与坏密码同为 `bad_credentials` 不可区分。
- 密码长度按 UTF-8 字节校验（8–72，bcrypt 72 字节上限），必须在 Validate 方法里做而非 validator tag；产品策略常量写死表现层，不经配置。
- 表现层 dto 子包只被本模块 handler 引用。

## Handler 契约

- handler 裸 gin 签名 `func(*gin.Context)`：错误分支 `response.Fail(c, err)` 后立即 return，成功分支 `response.OK(c, data)`；绑定失败经 `response.InvalidBind` 归一 400；无数据负载端点统一 `response.Ack`。
- 不建返回值驱动的两段式适配器；漏写响应由 ProcessLog 运行时兜底 500。
- 错误响应契约 `{"code":"snake_case","message":"中文文案"}`；成功响应 data 直接作响应体，不包信封。

## 持久化

- 表结构唯一事实源是 `infrastructure/persistence/mysql/po` 的 gorm 标签（显式列名/类型/约束/索引 + `TableName()`）+ 启动时 `mysql.Migrate`（AutoMigrate）幂等同步，失败即退出；不设 SQL 迁移文件，不引入 schema 第二事实源。
- 新增表 = po 加结构体 + Migrate 清单加一行。AutoMigrate 只增不改不删；收窄列、删列需人工 SQL 并在 po 注释留痕。
- domain 结构体禁止任何序列化/ORM 标签；`created_at` 由应用侧生成随 INSERT 携带，不回读；DSN 保持 `parseTime=true`。

## 配置与日志

- 全部配置走工作目录 `config.yaml`（模板 `config.example.yaml`），环境变量不是配置来源；密钥入文件靠文件权限管控；`jwt_secret` 必填，缺失 Fatal。
- 日志用 zap + `applog.L()` 全局单 logger，不包 Logger 接口；过程日志由表现层中间件输出单行摘要，业务过程数据经 context kv 追加，请求中途不输出独立日志行；请求标识回显响应头 `X-Request-Id`。

## 认证

- JWT 双令牌：access token 无状态不可吊销（分钟级）；refresh token 只存 SHA-256 哈希，HttpOnly Cookie（`Path=/api/auth`、`SameSite=Strict`）下发，登出即删行。
- logout 无 Cookie 幂等成功；服务端删除失败返回受控 5xx、不清 Cookie。
- refresh 接口仅当前语义为 `unauthorized` 时清 Cookie；数据库故障等保留凭据。

## 外部服务隔离

与持久化划界：自有数据技术（存储与缓存，如 mysql、redis）遵循仓储模式，住 `infrastructure/persistence/<技术>/`（见「持久化」章节）；外部服务（LLM、支付、OSS 对象存储等）遵循 port + adapter，住 `infrastructure/acl/`——即使外部服务也存数据（如 OSS），它是别人的系统，归 acl。两种隔离形态各自成章，互不隶属。

- **port 在业务侧**：用例语义的 port 定义在 `application/<模块>`（先例 `application/auth.TokenIssuer`）；领域语言的 port 定义在 `domain/<模块>`（先例 `user.Repository`）。port 按消费者需要的方法定义，不模仿外部服务 API；不设统一 port 包。
- **adapter 在 acl/**：按服务分子包 `acl/<service>/`（多供应商时子包内再分层），adapter 实现 port；外部库的 import 只允许出现在 acl/ 内。
- **三不漏**：外部库类型不漏入 domain/application；外部错误不裸抛——adapter 内出生点转换为 apperr；外部响应形状不直通 DTO，先转业务类型。
- **假实现与 port 同家**：port 的测试 fake 先写在消费方 `_test.go`；同一 port 出现第二个消费者时，提取到 port 所在包的 `fakes/` 子包（先例 `domain/user/fakes`，按契约错误形态对齐真实实现）。不设跨 port 的统一假件包。
- 外部服务特定形状（流式、额度计量、多供应商选择等）不预先设计，随首个用例落地时随代码定。

## 新增业务模块动作清单

1. `domain/<mod>/`：实体、值对象、仓储契约接口。
2. `infrastructure/persistence/mysql/po/`：表结构体 + Migrate 清单一行；`mapper.go` 转换；仓储实现 + 仓储边界分类。
3. `application/<mod>/`：Service（用例），多值产出进 `dto/`。
4. `presentation/api/<mod>/`：handler.go + dto/（含 Validate）；router 注册一行；main 组装一行。
5. 错误码：`kernel/apperr/codes.go` 声明 + errTable 登记（内部事实码除外）+ `backend/docs/api.md` 补表。

## 测试与验证

- port 假件规则见「外部服务隔离」：首用例写在 `_test.go`，第二消费者出现时提取到 port 所在包的 `fakes/`；假件错误形态对齐真实实现（如 email_taken、(nil,nil) 未命中）。
- 改动后 `gofmt` + 受影响包 `go test`；公共契约/跨包改动 `go test ./...`，条件允许加 `go vet ./...`。
- 行为变更同步 `backend/docs/api.md`。
