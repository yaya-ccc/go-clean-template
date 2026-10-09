// Package dto 集中认证模块表现层的 DTO：请求体与响应视图的协议契约。
package dto

import (
	"time"

	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/kernel/apperr"
)

// RegisterRequest 只描述协议形状；认证内容规则在应用调用链内统一保证。
type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
}

// Validate 只守协议形状，不重复领域内容规则，也不改变请求字段。
func (r *RegisterRequest) Validate() error {
	if r == nil {
		return apperr.ErrInvalidInput.With("register body empty")
	}
	return nil
}

// LoginRequest 不在协议层拒绝坏邮箱，由登录用例保持凭据语义不可区分。
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Validate 不检查凭据内容，避免协议入口绕开登录用例的防枚举规则。
func (r *LoginRequest) Validate() error {
	if r == nil {
		return apperr.ErrInvalidInput.With("login body empty")
	}
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
