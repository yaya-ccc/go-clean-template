// Package router 构建 gin 引擎并集中注册全部路由；依赖由 main 组装后注入。
// 业务路由统一挂 /api 前缀；/healthz 属基础设施探针，独立于 /api。
package router

import (
	"context"
	"database/sql"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	userapp "go-clean-template/backend/internal/application/user"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/kernel/applog"
	authpres "go-clean-template/backend/internal/presentation/api/auth"
	userpres "go-clean-template/backend/internal/presentation/api/users"
	"go-clean-template/backend/internal/presentation/middleware"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
)

// Deps 是路由注册所需的全部依赖，由 main 组装注入。
type Deps struct {
	DB         *sql.DB
	AuthSvc    *authapp.Service
	UserSvc    *userapp.Service
	RefreshTTL time.Duration // refresh cookie 有效期，与服务端 RefreshTTL 一致
}

// New 组装 gin 引擎并注册全部路由；依赖缺失视为组装错误，直接返回错误。
func New(d Deps) (*gin.Engine, error) {
	if d.DB == nil {
		return nil, apperr.ErrInternal.With("router.New: db must not be nil")
	}
	if d.AuthSvc == nil {
		return nil, apperr.ErrInternal.With("router.New: auth service must not be nil")
	}
	if d.UserSvc == nil {
		return nil, apperr.ErrInternal.With("router.New: user service must not be nil")
	}
	r := gin.New()
	r.Use(middleware.ProcessLog(applog.L()), middleware.ZapRecovery(applog.L())) // panic 恢复走 zap + 过程日志摘要行；设计见 docs/日志设计.md

	// 框架级错误同样走统一 JSON 出口。
	r.NoRoute(func(c *gin.Context) { response.Fail(c, apperr.ErrRouteNotFound.With("no route")) })
	r.HandleMethodNotAllowed = true
	r.NoMethod(func(c *gin.Context) { response.Fail(c, apperr.ErrMethodNotAllowed.With("no method")) })

	r.GET("/healthz", healthHandler{db: d.DB}.handle)

	api := r.Group("/api")
	if err := authpres.Register(api.Group("/auth"), d.AuthSvc, d.RefreshTTL); err != nil {
		return nil, err
	}
	if err := userpres.Register(api.Group("/users", middleware.RequireAuth(d.AuthSvc)), d.UserSvc); err != nil {
		return nil, err
	}

	return r, nil
}

// healthHandler 探测数据库连通性，供部署侧健康检查。
type healthHandler struct {
	db *sql.DB
}

// handle 对 /healthz 的处理：2 秒内 Ping 通返回 200 {"status":"ok"}（response.Ack），
// 失败返回 service_unavailable（503）。
func (h healthHandler) handle(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.PingContext(ctx); err != nil {
		// 健康检查失败是明确的服务不可用，不是未分类内部错误；探针与告警按 503 处理。
		if ctx.Err() != nil {
			response.Fail(c, apperr.ErrOperationTimeout.With("healthz ping", apperr.WithCause(ctx.Err())))
			return
		}
		response.Fail(c, apperr.ErrServiceUnavailable.With("healthz ping", apperr.WithCause(err)))
		return
	}
	response.Ack(c)
}
