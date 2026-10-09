package apperr

// 本表只登记稳定的对外语义。系统失败统一 internal_error，
// 技术类别通过静态 op 与 cause 留在日志，不再维护内部事实码。
const (
	// ErrInvalidInput 表示协议形状或业务输入不满足规则，不对外暴露字段细节。
	ErrInvalidInput ErrCode = "invalid_input"
	// ErrUnauthorized 表示凭据缺失、无效、过期或其关联用户已不存在。
	ErrUnauthorized ErrCode = "unauthorized"
	// ErrInternal 是所有系统失败与未知错误的统一语义；原因仅供日志排障。
	ErrInternal ErrCode = "internal_error"
	// ErrRouteNotFound 表示未注册的请求路径。
	ErrRouteNotFound ErrCode = "route_not_found"
	// ErrMethodNotAllowed 表示路径存在但请求方法不被支持。
	ErrMethodNotAllowed ErrCode = "method_not_allowed"
	// ErrBadCredentials 统一登录语境中的邮箱不存在与密码错误，防止用户枚举。
	ErrBadCredentials ErrCode = "bad_credentials"
	// ErrEmailTaken 表示邮箱唯一索引已被占用，由仓储边界直接分类。
	ErrEmailTaken ErrCode = "email_taken"
)
