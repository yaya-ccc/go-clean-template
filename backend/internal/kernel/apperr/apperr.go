// Package apperr 提供全项目统一的错误类型：稳定 code + 受控 detail 与原始因。
// 它是唯一允许被 domain/application/presentation 共同依赖的错误机制，
// 自身零业务依赖；对外用户文案与 HTTP 状态码由表现层翻译表维护。
package apperr

// ErrCode 是错误码的具名类型：snake_case 字符串，分支判断与翻译表查找的唯一依据。
// 声明集中在 codes.go 单一注册表；调用点不再手写 code 字符串。
type ErrCode string

// Error 让 ErrCode 可作为 errors.Is 的匹配目标；实际错误用 With 构造以保留操作上下文。
func (c ErrCode) Error() string { return string(c) }

// With 从错误码派生带操作上下文的项目错误；opts 附加原始因或类型化 detail。
// 同一 code 的所有实例同语义：errors.Is 按码命中，跨指针匹配。
func (c ErrCode) With(op string, opts ...Option) *Error {
	e := &Error{Code: c, Message: op}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

// Error 是带稳定错误码的业务错误。Code 是分支判断依据（snake_case），
// Message 是开发者写定的静态英文操作上下文（如 mysql.user.create），
// 不是用户文案，也绝不拼接外部输入或底层 err.Error()。
// 实例发布后视为只读；detail 只允许不可变的小型值对象（见 detail.go）。
type Error struct {
	Code    ErrCode
	Message string
	cause   error
	detail  any
}

// Option 是构造错误的可选参数。
type Option func(*Error)

// Error 只输出当前 code 与静态上下文，不递归拼接原始 cause；
// 原始错误文本只经 WithCause 保留在链上，供 applog 在日志中输出原文排障。
func (e *Error) Error() string {
	if e.Message == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Message
}

// Unwrap 保留错误链，使 errors.Is/As 可穿透 apperr 包装。
func (e *Error) Unwrap() error { return e.cause }

// Is 实现同码即匹配：只要 target 也是同码错误（*Error 或裸 ErrCode）即视为同一错误，
// 消除「同一 code 多个指针」导致的 errors.Is 静默失败。
// 注意 errors.Is 会沿链匹配：应用包装后的错误仍可同时命中内层旧语义，
// 按当前语义分支应使用 CodeIs。
func (e *Error) Is(target error) bool {
	switch t := target.(type) {
	case *Error:
		return t != nil && e != nil && t.Code != "" && e.Code == t.Code
	case ErrCode:
		return e != nil && t != "" && e.Code == t
	}
	return false
}

// First 返回错误链中最外层的项目错误；用于只按当前语义层决策的场合。
// 项目主错误通道不使用 errors.Join 多分支树。
func First(err error) (*Error, bool) {
	var e *Error
	if !errorsAs(err, &e) || e == nil {
		return nil, false
	}
	return e, true
}

// CodeOf 返回最外层项目错误的 code；非项目错误返回空码。
func CodeOf(err error) ErrCode {
	if e, ok := First(err); ok {
		return e.Code
	}
	return ""
}

// CodeIs 判断 err 的当前语义（最外层项目错误）是否为 code。
// 用于外层行为决策：errors.Is 沿链匹配会误命中被降级包装的内层码，
// 需要按当前语义分支时应调用本函数而非比较 CodeOf 或 errors.Is。
func CodeIs(err error, code ErrCode) bool {
	return CodeOf(err) == code
}

// DetailOf 只读取当前语义层（最外层项目错误）的 detail，绝不向内搜索旧语义的 detail：
// 应用层未携带某类型 detail 时不能读到仓储层 detail，避免跨语义误用。
func DetailOf[T any](err error) (T, bool) {
	var zero T
	e, ok := First(err)
	if !ok {
		return zero, false
	}
	d, ok := e.detail.(T)
	return d, ok
}
