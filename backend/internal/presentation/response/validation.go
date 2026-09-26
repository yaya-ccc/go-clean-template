package response

import (
	"go-clean-template/backend/internal/kernel/apperr"
)

// InvalidBind 把 ShouldBindJSON 失败分类为受控的 invalid_input（保留 cause）；
// 具体原因只进日志，不向客户端透出字段细节。
func InvalidBind(err error) error {
	return apperr.ErrInvalidInput.With("bind request body", apperr.WithCause(err))
}
