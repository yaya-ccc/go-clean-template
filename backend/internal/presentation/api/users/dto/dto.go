// Package dto 集中用户资料模块表现层的 DTO：响应视图的协议契约。
package dto

import (
	"time"

	"go-clean-template/backend/internal/domain/user"
)

// UserResponse 对外暴露的用户视图，绝不含密码哈希。
type UserResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	CreatedAt string `json:"created_at"`
}

// NewUserResponse 组装用户资料响应（GET /api/users/me）。
func NewUserResponse(u *user.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Nickname:  u.Nickname,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
