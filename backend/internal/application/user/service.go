// Package user 编排已登录用户自身资料的查询用例。
package user

import (
	"context"

	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/kernel/apperr"
)

// Service 是用户资料用例的编排入口。
type Service struct {
	users user.Repository
}

// NewService 构造用户资料服务；仓储为必填，nil 视为组装错误。
func NewService(users user.Repository) (*Service, error) {
	if users == nil {
		return nil, apperr.ErrInternal.With("user.NewService: users repository must not be nil")
	}
	return &Service{users: users}, nil
}

// FindByID 按用户 ID 查询用户资料。
// 未命中产出事实码 user_not_found，由 /users/me handler 就地合码为 unauthorized：
// token 有效但用户已不存在意味着前端应走重新登录流程，而非 500/404。
func (s *Service) FindByID(ctx context.Context, id int64) (*user.User, error) {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, apperr.ErrUserNotFound.With("user deleted but token still valid")
	}
	return u, nil
}
