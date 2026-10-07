// Package user 编排已登录用户自身资料的查询用例。
package user

import (
	"context"

	"go-clean-template/backend/internal/application/user/dto"
	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/kernel/apperr"
)

// Service 是当前用户资料用例的编排入口。
type Service struct {
	users user.Repository
}

// NewService 将仓储缺失视为组装错误，不提供降级运行。
func NewService(users user.Repository) (*Service, error) {
	if users == nil {
		return nil, apperr.ErrInternal.With("user.NewService: users repository must not be nil")
	}
	return &Service{users: users}, nil
}

// GetMe 只接受已认证身份；用户已删除时应重新登录，不产出通用的查无用户语义。
func (s *Service) GetMe(ctx context.Context, in dto.GetMeInput) (*user.User, error) {
	u, err := s.users.FindByID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, apperr.ErrUnauthorized.With("user.me: user deleted but token still valid")
	}
	return u, nil
}
