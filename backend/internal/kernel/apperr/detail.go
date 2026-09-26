package apperr

import "errors"

// WithCause 附加原始因：保留 errors.Is/As 穿透能力与日志排障链；
// nil 输入无效果（链式可选项天然无 typed-nil 问题）。
func WithCause(err error) Option {
	return func(e *Error) {
		if err != nil {
			e.cause = err
		}
	}
}

// WithDetail 附加类型化 detail：只允许所属模块定义的小型不可变值对象，
// 不允许 map[string]any、请求体、实体、驱动错误或任何凭据。
func WithDetail(detail any) Option {
	return func(e *Error) { e.detail = detail }
}

// errorsAs 内部薄封装，避免在包文档中暴露对 errors 的重复 import 说明。
func errorsAs(err error, target **Error) bool { return errors.As(err, target) }
