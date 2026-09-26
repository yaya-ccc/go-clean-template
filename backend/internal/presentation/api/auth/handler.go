package auth

import (
	"net/http"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/api/auth/dto"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
)

// refreshCookieName 是刷新凭据 HttpOnly Cookie 的名称。
const refreshCookieName = "refresh_token"

// setRefreshCookie 以 HttpOnly Cookie 下发刷新凭据：限定 /api/auth 路径、
// SameSite=Strict 防 CSRF，有效期与服务端 RefreshTTL 对齐（由 main 注入）。
// Secure 未设，本地 http 开发不强制，上生产 TLS 后打开。
func setRefreshCookie(c *gin.Context, raw string, ttl time.Duration) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, raw, int(ttl.Seconds()), "/api/auth", "", false, true)
}

// clearRefreshCookie 清除客户端刷新凭据 Cookie。
func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/auth", "", false, true)
}

// handler 处理 /api/auth/* 的注册、登录、刷新与登出。
type handler struct {
	svc        *authapp.Service
	refreshTTL time.Duration
}

// Register 在 /api/auth 分组下注册本模块路由；svc 为必填，nil 视为组装错误。
func Register(g *gin.RouterGroup, svc *authapp.Service, refreshTTL time.Duration) error {
	if svc == nil {
		return apperr.ErrInternal.With("authpres.Register: service must not be nil")
	}
	h := handler{svc: svc, refreshTTL: refreshTTL}
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
	return nil
}

// register 绑定并校验请求体（校验全部在表现层 DTO），成功渲染用户视图。
func (h handler) register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.InvalidBind(err))
		return
	}
	if err := req.Validate(); err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.svc.Register(c.Request.Context(), req.Email, req.Password, req.Nickname)
	if err != nil {
		response.Fail(c, err)
		return
	}
	setRefreshCookie(c, res.Tokens.Refresh, h.refreshTTL)
	response.OK(c, dto.NewUserResponse(res.User, res.Tokens.Access))
}

// login 绑定请求体后走应用服务；坏邮箱在表现层放行，与坏密码同为 bad_credentials。
// 应用层产出的 user_not_found 在此就地合码为 bad_credentials：对外响应与密码错误
// 字节级一致，防用户枚举；时间侧信道由应用层 dummy bcrypt 拉平。
func (h handler) login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.InvalidBind(err))
		return
	}
	if err := req.Validate(); err != nil {
		response.Fail(c, err)
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if apperr.CodeIs(err, apperr.ErrUserNotFound) {
			err = apperr.ErrBadCredentials.With("login email not found", apperr.WithCause(err))
		}
		response.Fail(c, err)
		return
	}
	setRefreshCookie(c, res.Tokens.Refresh, h.refreshTTL)
	response.OK(c, dto.NewUserResponse(res.User, res.Tokens.Access))
}

// refresh 只有当前语义为 unauthorized（凭据确定无效/过期）时才清 Cookie；
// 数据库故障、保存失败、处理超时保留凭据，用户可在服务恢复后继续操作。
// 应用层产出的 refresh_token_not_found 在此就地合码为 unauthorized。
func (h handler) refresh(c *gin.Context) {
	// gin 的 Cookie 错误只有 http.ErrNoCookie（未携带），解码失败被其吞掉；
	// 无诊断信息可保留，故不作为 cause 附加，与空值同按 unauthorized 处理。
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		clearRefreshCookie(c) // 残留空值 Cookie 清掉，避免反复触发刷新
		response.Fail(c, apperr.ErrUnauthorized.With("no refresh cookie"))
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), raw)
	if err != nil {
		if apperr.CodeIs(err, apperr.ErrRefreshTokenNotFound) {
			err = apperr.ErrUnauthorized.With("refresh token not found", apperr.WithCause(err))
		}
		if apperr.CodeIs(err, apperr.ErrUnauthorized) { // 只看最外层语义：errors.Is 沿链匹配会误命中被降级包装的内层码
			clearRefreshCookie(c)
		}
		response.Fail(c, err)
		return
	}
	setRefreshCookie(c, pair.Refresh, h.refreshTTL)
	response.OK(c, dto.RefreshResponse{AccessToken: pair.Access})
}

// logout 契约：无 Cookie 幂等成功（不访问数据库）；
// 凭据不存在幂等成功；删除成功才清 Cookie；服务端删除失败返回受控 5xx，
// 不声称已吊销、不清 Cookie（保留客户端重试吊销的能力）。
// 成功渲染统一执行成功响应 response.Ack。
func (h handler) logout(c *gin.Context) {
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		clearRefreshCookie(c)
		response.Ack(c)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), raw); err != nil {
		response.Fail(c, err)
		return
	}
	clearRefreshCookie(c)
	response.Ack(c)
}
