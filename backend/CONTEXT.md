# backend/CONTEXT.md — 后端术语表

后端实现域的术语表，只定义概念；产品域术语（用户等）见根目录 `CONTEXT.md`，后端约束见 `backend/AGENTS.md`。

## Language

### 账号与认证（实现侧）

**令牌对（Token Pair）**:
一次签发同时产出的 access token 与 refresh token；领域形态为 domain/user.TokenPair。

**DTO**:
分层产出的命名数据类型，按层分属每模块的 dto 子包（application/auth/dto、presentation/api/auth/dto 等）。
应用层 DTO 是用例多于一个成功产出时收拢的结果（如 AuthResult），无 json tag、持有领域对象引用；单值与无产出用例不包装。
表现层 DTO 是协议视图（请求体 + 响应视图），带 json tag。
_Avoid_: 用例结果（旧称，已并入 DTO）、返回值对象、视图对象

### 过程日志

一次 HTTP 请求处理结束时由中间件输出的单行摘要，携带请求标识、基础字段与业务过程数据；请求中途不产生独立日志行。

### 错误体系

**错误码（Error Code）**:
对外错误响应中机器可读的稳定标识（snake_case 字符串，如 `email_taken`），前端按它分支处理；与 HTTP 状态码一一对应，由表现层翻译表统一维护。
_Avoid_: 错误号、数字码、错误枚举

**语义转译（Semantic Translation）**:
应用层将仓储基础错误在其用例语境下转成另一个错误码后返回的行为（如「token 有效但用户已删」转译为 `unauthorized`）；
差异语义用不同的码表达，而不是同一码在不同接口换状态码或文案。唯一冲突类错误不经语义转译，
由仓储边界分类直接产出。查询未命中不走错误通道：仓储返回 nil 指针，
应用层判空产出内部事实码，需要模糊化的接口（登录防用户枚举）由 handler 就地合码为对外码。

**业务错误码（Business Error Code）**:
模块用例语义的错误码（如 email_taken），与基础错误码一样集中注册于横切层错误包，
全项目单点声明；其产出点随错误性质而定——唯一冲突等事实可判定语义由仓储边界分类直接产出，
其余用例语义由应用层编排产出。另有内部事实子集（user_not_found、
refresh_token_not_found）：不入表现层公开翻译表，永不直接对外，泄漏时降级为内部错误。
_Avoid_: 业务码散落各模块应用层声明

**基础错误（Base Error）**:
跨模块通用的事实性错误（如数据库故障、内部失败），单一定义于横切层，全项目共用；
`database_error` 等系统故障码对外公开（500），细节只进日志诊断通道；查询未命中不再是
基础错误——它是 Find 类查询的正常产出（nil 指针），事实码由应用层按用例产出。
与业务错误码的分工：基础错误描述跨模块的技术事实，业务错误码描述单一用例的对外语义；
二者同注册表、同翻译表，产出职责不同。
_Avoid_: 通用错误、公共错误、底层错误

**请求标识（Request ID）**:
服务端为每个请求自生成的短随机标识，贯穿该请求全部日志输出并回显在响应头 X-Request-Id 中，用于跨日志行定位单次请求。
_Avoid_: trace id、链路 id

### 表现层结构

**业务模块包（Module Package）**:
表现层按业务模块划分的包（presentation/api/<模块>），承载该模块的路由注册、handler 与协议细节；互不 import。
_Avoid_: 业务部分、业务包

**协议公共件（HTTP Common）**:
表现层内跨模块共享的 HTTP 公共组件（router、response、middleware），不含业务语义、不 import 业务模块包；鉴权所需的应用服务由 router 注入。
_Avoid_: 系统部分、系统件、公共库

### 横切组件

**横切组件（Cross-cutting Component）**:
可被任一架构层安全依赖的基础能力，如错误体系、过程日志、配置。唯一的准入条件是零 import 依赖（不引入对业务包的代码依赖）；错误码注册表这类纯标识符登记不算业务逻辑承载，依赖具体技术选型（如日志库）不违规。
_Avoid_: 公共库、utils、common（无准入规则的大杂烩）

**防腐层（Anti-Corruption Layer）**:
外部服务（LLM、支付、短信等）与应用核心之间的隔离结构，由 port（定义在业务侧的抽象接口）与 adapter（infrastructure/acl/ 下的实现）组成；外部库的类型、错误与响应形状经它转换后才进入 domain/application。专指外部服务隔离，不含数据持久化（见「仓储边界分类」与「持久化模型」）。
_Avoid_: ACL 网关、适配层（泛称）、把 mysql 的隔离也叫防腐层

**仓储边界分类（Repository Boundary Classification）**:
仓储实现在其边界上把驱动错误直接分类为业务码或基础码的行为（如邮箱唯一索引冲突 → email_taken，未知故障 → database_error）；调用方透传即可，不再二次翻译。
_Avoid_: 防腐层（该词专属外部服务隔离）、错误归一化（只覆盖 database_error 一路）

### 持久化边界

**持久化模型（Persistence Model）**:
基础设施层表形状的集中定义（如 mysql 存储下的 po 包），是存储形态与领域模型之间的转换边界；转换由仓储完成，仓储契约与应用层不经手它。
_Avoid_: model、entity、数据模型；PO 不作交流用语，仅允许作为代码标识符（如 userPO）
