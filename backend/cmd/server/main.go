// go-clean-template 服务入口：加载配置、组装依赖、启动 HTTP 服务并优雅停机。
package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	userapp "go-clean-template/backend/internal/application/user"
	"go-clean-template/backend/internal/infrastructure/persistence/mysql"
	"go-clean-template/backend/internal/kernel/applog"
	"go-clean-template/backend/internal/kernel/config"
	"go-clean-template/backend/internal/presentation/router"

	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	applog.Init(applog.Config{Format: cfg.Log.Format, Level: cfg.Log.Level})
	logger := applog.L()
	g, err := mysql.Open(cfg.MySQLDSN)
	if err != nil {
		logger.Fatal("打开数据库失败", zap.Error(err))
	}
	sqlDB, err := mysql.SQLDB(g)
	if err != nil {
		logger.Fatal("取回数据库句柄失败", zap.Error(err))
	}
	defer sqlDB.Close()

	// schema 唯一来源是 po + AutoMigrate；启动时同步，失败即退出。
	if err := mysql.Migrate(g); err != nil {
		logger.Fatal("数据库迁移失败", zap.Error(err))
	}

	// 组装根：repo → 应用服务 → 路由依赖，表现层只收现成依赖；依赖缺失即启动失败。
	userRepo, err := mysql.NewUserRepo(g)
	if err != nil {
		logger.Fatal("构造用户仓储失败", zap.Error(err))
	}
	refreshRepo, err := mysql.NewRefreshTokenRepo(g)
	if err != nil {
		logger.Fatal("构造刷新凭据仓储失败", zap.Error(err))
	}
	authSvc, err := authapp.NewService(
		userRepo,
		refreshRepo,
		authapp.NewJWTIssuer(cfg.JWTSecret),
		authapp.Config{
			AccessTTL:  cfg.AccessTTL,
			RefreshTTL: cfg.RefreshTTL,
		},
	)
	if err != nil {
		logger.Fatal("构造认证服务失败", zap.Error(err))
	}
	userSvc, err := userapp.NewService(userRepo)
	if err != nil {
		logger.Fatal("构造用户服务失败", zap.Error(err))
	}
	handler, err := router.New(router.Deps{
		DB:         sqlDB,
		AuthSvc:    authSvc,
		UserSvc:    userSvc,
		RefreshTTL: cfg.RefreshTTL,
	})
	if err != nil {
		logger.Fatal("组装路由失败", zap.Error(err))
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		logger.Info("listening", zap.String("addr", cfg.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("HTTP 服务异常退出", zap.Error(err))
		}
	}()

	<-ctx.Done() // 等待 SIGINT/SIGTERM
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("优雅停机未完成，强制结束", zap.Error(err))
	}
	logger.Info("server stopped")
}
