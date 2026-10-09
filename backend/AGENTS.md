# backend/AGENTS.md — 后端最终约束

本文件是后端的最终约束，面向在 `backend/` 下工作的开发人员与 AI 助手；后端实现术语见 `backend/CONTEXT.md`，产品域术语见根 `CONTEXT.md`。设计过程的历史决策记录（ADR）已删除，其结论已收拢进本文与相关注释。

## 目录骨架

```
backend/internal/
├── domain/                 # 领域层：实体、值对象、业务规则、仓储契约；按模块分包
│   └── user/               # User、TokenPair、邮箱归一化与密码/昵称规则、Repository 接口；fakes/ 共享测试假件
├── application/            # 应用层：用例编排；按模块分包
│   ├── auth/               # Service（用例入口）、TokenIssuer 抽象、dto/（Input 与 AuthResult 等）
│   └── user/
├── infrastructure/
│   ├── acl/                # 外部服务防腐适配器（port 在业务侧，见「外部服务隔离」章节）
│   └── persistence/        # 自有数据技术（存储与缓存）的仓储实现；按技术分子包
│       └── mysql/          # 仓储实现 + po/（表形状）+ mapper.go（唯一转换点）+ errors.go（仓储边界分类）
├── presentation/
│   ├── api/                # 业务模块包：auth/ users/ 各含 handler.go（路由注册+handler+cookie 等协议细节）、dto/
│   ├── response/           # 错误翻译表 errTable + OK/Fail/Ack 渲染（request_id 参数注入）
│   ├── middleware/         # ProcessLog、RequireAuth、ZapRecovery + Bind/Handle + context.go 请求值存取
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
- presentation：import application、domain（仅 DTO 视图组装需要）、kernel；禁止 infrastructure、cmd。
- presentation 内部：业务模块包（api/ 下按模块分包）互不 import；协议公共件（router、response、middleware）不 import 业务模块包，鉴权所需的应用服务由 router 注入。
- kernel：零业务 import 依赖；可依赖具体技术选型（zap、go-yaml）。
- cmd：可以 import 一切，是唯一允许「具体实现接线」的位置。

「在哪写什么」：

- 业务概念/规则/不变量 → `domain/<模块>`：实体与值对象零标签（无 json/gorm tag），仓储契约是接口。
- 用例编排 → `application/<模块>`：公开用例统一 `(ctx, XxxInput) (ResultType, error)`，有输入使用本模块 `dto/` 的用例名 + Input，无输入只传 ctx；多于一个成功产出收拢 Result DTO，单值直接返回，无产出只返回 `error`。Input/Result 无 json tag，可持有领域对象。构造函数、port 方法、配置访问器与内部辅助函数不套此契约。协议细节（cookie、状态码）留在表现层。
- 请求 DTO → Input 的纯转换放本模块表现层 `mapper.go`；转换不承担校验或归一化，handler 调用预制映射。仅来自路径、Cookie 或认证上下文的简单输入可直接具名组装。
- 表访问/SQL/驱动错误翻译 → `infrastructure/persistence/mysql`：表形状只在 `po/`，po↔domain 转换只在 `mapper.go`；确实表达不了的查询允许仓储内 Raw SQL，但 SQL 不出仓储。
- HTTP 协议（路由、绑定、校验、渲染、cookie、鉴权）→ `presentation/api/<模块>`；新路由在 `router` 集中注册并决定鉴权分组。
- 依赖组装 → 只在 `cmd/server/main.go` 与 `router.New(Deps)`：仓储、应用服务都在 main 构造后注入；模块包与 router 只收现成依赖，nil 依赖视为组装错误直接返回错误。

## 错误体系

- 错误值用 `apperr.ErrCode.With(op, ...)` 构造；标准库/第三方错误在调用边界分类并转换，跨模块/分层前必须是 apperr。包内辅助函数可返回原始原因供调用点分类；已分类错误直接透传，不为层级重复包装。
- `kernel/apperr/codes.go` 只登记稳定对外语义，每码显式声明为一个 `ErrCode` 字符串常量；调用点引用常量。`presentation/response.errTable` 维护一码一状态一文案；注册表与翻译表全集覆盖由测试守护。未登记码、裸错误统一降级 `internal_error`，日志当前公开码与实际响应一致，原始 cause 保留。
- **系统失败统一 500 `internal_error`**：数据库故障、依赖超时、服务不可用与探针失败不另设技术码。静态 op 和 cause 承担内部归因，不承诺操作未生效、不自动重试写操作。
- **用例决定最终业务语义，表现层只渲染**：登录的邮箱不存在与密码错误同产 `bad_credentials`，刷新凭据不存在/过期同产 `unauthorized`，当前用户已删除同产 `unauthorized`。差异只留静态 op，不维护用户/刷新凭据内部事实码，也不由 handler 合码。
- 仓储 Find 未命中返回 `(nil, nil)`，应用层按用例语境处理；明确的邮箱唯一索引冲突由仓储直接产出 `email_taken`，未知索引冲突归 `internal_error`，不猜字段、不做第二次语义翻译。
- `apperr.CodeIs` 判断最外层当前语义（如是否清 Cookie）；`errors.Is/As` 沿链检查原因/内层语义（如标准库取消与超时）。二者不可互换。主错误通道保持单链，不用 `errors.Join`。
- op 是静态操作上下文，不拼外部输入或 `err.Error()`；用户文案只来自翻译表，cause 只进日志。领域规则可直接产出 apperr，不额外发明裸哨兵映射层。
- 请求上下文已结束时不再写 JSON，ProcessLog 不补写 500、仍记录 `canceled`；仅依赖子操作取消而请求仍有效时，正常返回 500。取消不意味着已发生的业务写入被回滚。

## 校验与 DTO

- **协议形状 → 表现层**：JSON 可绑定性、必填、分页等协议骨架由 binding tag 与请求 DTO 的 `Validate() error` 保证。失败统一 400 `invalid_input`，无字段级详情；请求 DTO 不校验业务内容、不归一化业务字段。
- **业务内容 → 领域/应用调用链**：领域维护规则单一真相，应用层调用规则、完成归一化并判断用例条件；直接从任务/CLI/测试调用用例也不能绕过内容规则。不在 HTTP DTO 再复刻一份。
- 认证示例：邮箱规范化与合法性用 `domain/user.NormalizeEmail`；注册在应用层拒绝坏邮箱，登录只归一化、坏邮箱自然查询失败，与坏密码统一为 `bad_credentials`。未命中登录执行固定 dummy bcrypt，维持防枚举时间防护。
- 密码 8–72 UTF-8 字节、昵称最多 64 字符的规则在 `domain/user`，不经配置、不用 tag 重复。应用层派生默认昵称后也校验最终昵称；邮箱前缀超长时需显式提供合规昵称。
- 应用层 Input 与表现层请求 DTO 分离，不直接传 HTTP DTO 给用例；表现层 `dto/` 只被所属模块引用。

## Handler 契约

- 有 JSON 请求体：`func(*gin.Context, *dto.XxxRequest) (any, error)`，注册用 `middleware.Bind[dto.XxxRequest]`；无请求体：`func(*gin.Context) (any, error)`，注册用 `middleware.Handle`。handler 只返回协议响应 DTO 或 `nil` 与 `error`，不手写响应正文、不返回 typed-nil DTO。
- Bind 收拢 ShouldBindJSON（含 tag 校验）+ DTO Validate；适配器调用后统一渲染：错误 → Fail，`nil, nil` → Ack，其余 → OK。错误优先于成功产出；Cookie 和响应头由 handler 在返回前操作。
- response 不自行读取请求标识，渲染调用方经参数显式注入。请求值的键与存取收拢在 `middleware/context.go`；不新增跨层请求上下文包。
- 错误响应固定 `{"code":"snake_case","message":"中文文案","request_id":"16hex"}`，标识与响应头及日志同源，覆盖绑定、鉴权、404/405、panic 与框架兜底；成功 data 直接作响应体，不包信封、不增加 request_id。
- 无负载动作统一 200 `{"status":"ok"}`，不造空 DTO；取消或已提交的响应不追加 JSON。ProcessLog 的漏响应 500 仅兜底框架级裸 handler。

## 持久化

- 表结构唯一事实源是 `infrastructure/persistence/mysql/po` 的 gorm 标签（显式列名/类型/约束/索引 + `TableName()`）+ 启动时 `mysql.Migrate`（AutoMigrate）幂等同步，失败即退出；不设 SQL 迁移文件，不引入 schema 第二事实源。
- 新增表 = po 加结构体 + Migrate 清单加一行。AutoMigrate 只增不改不删；收窄列、删列需人工 SQL 并在 po 注释留痕。
- domain 结构体禁止任何序列化/ORM 标签；`created_at` 由应用侧生成随 INSERT 携带，不回读；DSN 保持 `parseTime=true`。

## 配置与日志

- 全部配置走工作目录 `config.yaml`（模板 `config.example.yaml`），环境变量不是配置来源；密钥入文件靠文件权限管控；`jwt_secret` 必填，缺失 Fatal。
- 日志用 zap + `applog.L()` 全局单 logger，不包 Logger 接口；过程日志由表现层中间件输出单行摘要，业务过程数据经标准 context kv 追加。请求标识由服务端生成 16 hex，不接纳上游标识；入口写入一次 gin 请求值，响应头 `X-Request-Id`、错误体、摘要与 panic 诊断共用。panic 堆栈诊断是单行摘要之外的既有例外。

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
3. `application/<mod>/`：Service（用例），Input 与多值 Result 进 `dto/`；内容规则经领域保证。
4. `presentation/api/<mod>/`：handler.go + dto/（协议形状 Validate）+ mapper.go；路由用 Bind/Handle，router 与 main 组装。
5. 错误码：稳定业务/协议语义在 `kernel/apperr/codes.go` 声明 + errTable 登记 + `backend/docs/api.md` 补表；技术原因用 internal_error + cause。

## 测试与验证

- port 假件规则见「外部服务隔离」：首用例写在 `_test.go`，第二消费者出现时提取到 port 所在包的 `fakes/`；假件错误形态对齐真实实现（如 email_taken、(nil,nil) 未命中）。
- 改动后 `gofmt` + 受影响包 `go test`；公共契约/跨包改动 `go test ./...`，条件允许加 `go vet ./...`。
- 契约检查至少覆盖直接调用用例的内容校验、绑定/渲染适配器、错误码注册表全集、请求标识同源、取消出口、最外层 Cookie 决策与系统失败不泄漏；行为变更同步 `backend/docs/api.md`。
