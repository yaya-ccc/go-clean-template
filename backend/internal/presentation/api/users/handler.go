package users

import (
	"go-clean-template/backend/internal/application/user"
	userdto "go-clean-template/backend/internal/application/user/dto"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/api/users/dto"
	"go-clean-template/backend/internal/presentation/middleware"

	"github.com/gin-gonic/gin"
)

type handler struct {
	users *user.Service
}

// Register 只接受现成用例依赖，鉴权由 router 在所属分组挂载。
func Register(g *gin.RouterGroup, svc *user.Service) error {
	if svc == nil {
		return apperr.ErrInternal.With("userpres.Register: service must not be nil")
	}
	h := handler{users: svc}
	g.GET("/me", middleware.Handle(h.me))
	return nil
}

func (h handler) me(c *gin.Context) (any, error) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		return nil, apperr.ErrUnauthorized.With("no user in context")
	}
	u, err := h.users.GetMe(c.Request.Context(), userdto.GetMeInput{UserID: userID})
	if err != nil {
		return nil, err
	}
	return dto.NewUserResponse(u), nil
}
