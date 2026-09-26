package apperr

// 本文件是全项目错误码的唯一注册表：所有对外契约码在此单点声明，
// 按基础码（协议通用码 + 仓储层事实码）与业务码（各模块用例语义）分区。

// 哨兵是裸 ErrCode（满足 error 接口），需要操作上下文时用 With 派生；
// 派生实例与哨兵同码，errors.Is 跨形态命中。
const (
	// ErrInvalidInput 表示请求参数不满足校验规则；任何模块的参数校验都可引用。
	ErrInvalidInput ErrCode = "invalid_input"
	// ErrUnauthorized 表示凭据缺失或无效（未携带 token、token 无效、refresh 过期等）。
	ErrUnauthorized ErrCode = "unauthorized"
	// ErrInternal 表示未分类的内部失败，是所有未知错误的统一归宿；
	// WithCause 携带的因果实链仅供日志排障，对外恒定 500。
	ErrInternal ErrCode = "internal_error"
	// ErrDatabase 表示数据库起源的故障（连接失败、驱动报错、未识别的写入拒绝等），
	// 是仓储层产出的基础事实；对外 500，驱动码等细节只进日志诊断通道。
	ErrDatabase ErrCode = "database_error"
	// ErrServiceUnavailable 表示明确的服务不可用（如健康检查探测失败），对外 503。
	ErrServiceUnavailable ErrCode = "service_unavailable"
	// ErrOperationTimeout 表示本项目处理预算或依赖操作超时，对外 503；
	// 不承诺操作未生效，不自动重试。
	ErrOperationTimeout ErrCode = "operation_timeout"
	// ErrRequestCanceled 是内部码：HTTP 边界须结合请求上下文判断，
	// 通常记录中止而不继续写 JSON，不作为对外契约。
	ErrRequestCanceled ErrCode = "request_canceled"
	// ErrRouteNotFound 表示请求路径未注册任何路由，对外 404（表现层框架码）。
	ErrRouteNotFound ErrCode = "route_not_found"
	// ErrMethodNotAllowed 表示路径存在但 HTTP 方法未注册，对外 405（表现层框架码）。
	ErrMethodNotAllowed ErrCode = "method_not_allowed"
)

// 业务码：模块用例语义的错误码，集中注册于本表供全项目引用；
// 其语义产出点（什么场景产出该码）随各模块仓储边界分类或用例编排产出。

const (
	// ErrBadCredentials 表示密码错误，或登录接口对用户不存在的就地合码结果；
	// 对外不区分二者（防用户枚举)）。
	ErrBadCredentials ErrCode = "bad_credentials"
	// ErrEmailTaken 表示邮箱已被注册（auth 注册）。
	ErrEmailTaken ErrCode = "email_taken"
	// ErrUserNotFound 表示按条件未查到用户，由应用层在仓储 Find 返回 nil 指针时产出；
	// 刻意不入表现层公开翻译表——登录接口须就地合码为 bad_credentials 防用户枚举，
	// handler 忘记合码时表现层按未登记码降级 500，泄漏方向的失败模式是报错而非暴露。
	ErrUserNotFound ErrCode = "user_not_found"
	// ErrRefreshTokenNotFound 表示按哈希未查到刷新凭据，由应用层在 Find 返回 nil 时产出；
	// 不入公开翻译表，refresh 接口就地合码为 unauthorized。
	ErrRefreshTokenNotFound ErrCode = "refresh_token_not_found"
)
