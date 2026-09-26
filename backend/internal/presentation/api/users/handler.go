package users

import (
	"go-clean-template/backend/internal/application/user"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/api/users/dto"
	"go-clean-template/backend/internal/presentation/middleware"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
)

// handler 处理已登录用户自身的资料接口。
type handler struct {
	users *user.Service
}

// Register 在分组下注册本模块路由；分组由 router 挂好 RequireAuth 后传入，svc 为必填。
func Register(g *gin.RouterGroup, svc *user.Service) error {
	if svc == nil {
		return apperr.ErrInternal.With("userpres.Register: service must not be nil")
	}
	h := handler{users: svc}
	g.GET("/me", h.me)
	return nil
}

// me 返回当前登录用户信息（GET /api/users/me）。
// 应用层产出的 user_not_found 在此就地合码为 unauthorized：token 有效但用户已删
// 意味着应走重新登录，而非 404/500。
func (h handler) me(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, apperr.ErrUnauthorized.With("no user in context"))
		return
	}
	u, err := h.users.FindByID(c.Request.Context(), userID)
	if err != nil {
		if apperr.CodeIs(err, apperr.ErrUserNotFound) {
			err = apperr.ErrUnauthorized.With("user deleted but token still valid", apperr.WithCause(err))
		}
		response.Fail(c, err)
		return
	}
	response.OK(c, dto.NewUserResponse(u))
}
