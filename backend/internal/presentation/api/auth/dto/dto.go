// Package dto 集中认证模块表现层的 DTO：请求体与响应视图的协议契约。
package dto

import (
	"time"

	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/kernel/apperr"
)

// 密码长度策略（产品常量写死于表现层，不经配置）：8 为产品最短要求，72 为 bcrypt 输入上限。
// 按 UTF-8 字节计数而非字符：validator 标签按字符数，72 个多字节字符会穿透到 bcrypt 报内部错误。
const (
	minPasswordBytes = 8
	maxPasswordBytes = 72
)

// RegisterRequest 注册请求体；自持 Validate 完成全部校验与邮箱归一化。
type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname" binding:"max=64"`
}

// Validate 校验注册请求体并就地归一化：邮箱格式与长度经 domain/user 单一真相判定，
// 密码按字节限制在 [8,72]；失败原因只进日志（op），不透出到响应。
func (r *RegisterRequest) Validate() error {
	if r == nil {
		return apperr.ErrInvalidInput.With("register body empty")
	}
	email, ok := user.NormalizeEmail(r.Email)
	if !ok {
		return apperr.ErrInvalidInput.With("register email invalid")
	}
	r.Email = email
	if n := len(r.Password); n < minPasswordBytes {
		return apperr.ErrInvalidInput.With("register password too short")
	} else if n > maxPasswordBytes {
		return apperr.ErrInvalidInput.With("register password too long")
	}
	return nil
}

// LoginRequest 登录请求体；坏邮箱不在校验层拒绝，放行后由查询自然失败，
// 保持与坏密码同为 bad_credentials、不可区分。
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Validate 仅做邮箱归一化写回；除空体外恒返回 nil。
func (r *LoginRequest) Validate() error {
	if r == nil {
		return apperr.ErrInvalidInput.With("login body empty")
	}
	r.Email, _ = user.NormalizeEmail(r.Email)
	return nil
}

// UserResponse 对外暴露的用户视图，绝不含密码哈希。
type UserResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Nickname    string `json:"nickname"`
	CreatedAt   string `json:"created_at"`
	AccessToken string `json:"access_token,omitempty"`
}

// RefreshResponse 是刷新接口的成功响应视图，仅携带新的 access token；
// 新 refresh 凭据经 HttpOnly Cookie 下发，不经响应体。
type RefreshResponse struct {
	AccessToken string `json:"access_token"`
}

// NewUserResponse 组装用户响应；带 access token 时用于注册/登录。
func NewUserResponse(u *user.User, accessToken string) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		Nickname:    u.Nickname,
		CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		AccessToken: accessToken,
	}
}
