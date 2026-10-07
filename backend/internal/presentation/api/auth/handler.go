package auth

import (
	"net/http"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/api/auth/dto"
	"go-clean-template/backend/internal/presentation/middleware"

	"github.com/gin-gonic/gin"
)

const refreshCookieName = "refresh_token"

// Cookie 限定认证路径且不向 JavaScript 暴露；生产 TLS 部署须启用 Secure。
func setRefreshCookie(c *gin.Context, raw string, ttl time.Duration) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, raw, int(ttl.Seconds()), "/api/auth", "", false, true)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/auth", "", false, true)
}

type handler struct {
	svc        *authapp.Service
	refreshTTL time.Duration
}

// Register 将 JSON 绑定和响应渲染交给统一适配器，依赖缺失属于组装错误。
func Register(g *gin.RouterGroup, svc *authapp.Service, refreshTTL time.Duration) error {
	if svc == nil {
		return apperr.ErrInternal.With("authpres.Register: service must not be nil")
	}
	h := handler{svc: svc, refreshTTL: refreshTTL}
	g.POST("/register", middleware.Bind[dto.RegisterRequest](h.register))
	g.POST("/login", middleware.Bind[dto.LoginRequest](h.login))
	g.POST("/refresh", middleware.Handle(h.refresh))
	g.POST("/logout", middleware.Handle(h.logout))
	return nil
}

func (h handler) register(c *gin.Context, req *dto.RegisterRequest) (any, error) {
	res, err := h.svc.Register(c.Request.Context(), toRegisterInput(req))
	if err != nil {
		return nil, err
	}
	setRefreshCookie(c, res.Tokens.Refresh, h.refreshTTL)
	return dto.NewUserResponse(res.User, res.Tokens.Access), nil
}

func (h handler) login(c *gin.Context, req *dto.LoginRequest) (any, error) {
	res, err := h.svc.Login(c.Request.Context(), toLoginInput(req))
	if err != nil {
		return nil, err
	}
	setRefreshCookie(c, res.Tokens.Refresh, h.refreshTTL)
	return dto.NewUserResponse(res.User, res.Tokens.Access), nil
}

// 只看最外层 unauthorized 决定清 Cookie，系统失败保留凭据用于恢复后的重试。
func (h handler) refresh(c *gin.Context) (any, error) {
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		clearRefreshCookie(c)
		return nil, apperr.ErrUnauthorized.With("no refresh cookie")
	}
	pair, err := h.svc.Refresh(c.Request.Context(), toRefreshInput(raw))
	if err != nil {
		if apperr.CodeIs(err, apperr.ErrUnauthorized) {
			clearRefreshCookie(c)
		}
		return nil, err
	}
	setRefreshCookie(c, pair.Refresh, h.refreshTTL)
	return dto.RefreshResponse{AccessToken: pair.Access}, nil
}

// 无 Cookie 或凭据已不存在时幂等成功；存储删除失败不得声称已吊销，也不清 Cookie。
func (h handler) logout(c *gin.Context) (any, error) {
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		clearRefreshCookie(c)
		return nil, nil
	}
	if err := h.svc.Logout(c.Request.Context(), toLogoutInput(raw)); err != nil {
		return nil, err
	}
	clearRefreshCookie(c)
	return nil, nil
}
