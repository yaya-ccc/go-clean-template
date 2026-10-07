// Package middleware 是表现层共享的 HTTP 中间件与请求上下文存取：
// 过程日志、panic 恢复与 Bearer 认证，不含业务语义。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"go-clean-template/backend/internal/application/auth"
	authdto "go-clean-template/backend/internal/application/auth/dto"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/kernel/applog"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// requestIDHeader 是回显请求标识的响应头。
const requestIDHeader = "X-Request-Id"

// bearerPrefix 是 Authorization 头中 access token 的前缀。
const bearerPrefix = "Bearer "

// ProcessLog 是过程日志中间件：生成请求标识、挂过程数据收集器，请求结束时输出单行摘要。
// 级别映射：2xx/3xx Info、4xx Warn、5xx Error；/healthz 成功降噪跳过、失败仍记录。
// 错误链经 applog.ErrorFields 编码：error_chain + 链尾 cause 原文。
func ProcessLog(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := newRequestID()
		c.Header(requestIDHeader, requestID)
		withRequestID(c, requestID)
		c.Request = c.Request.WithContext(applog.NewCollector(c.Request.Context()))
		start := time.Now()

		c.Next()

		// 只兜底仍有效的框架级裸 handler，取消请求不补写 JSON。
		if !c.Writer.Written() && c.Request.Context().Err() == nil {
			response.Fail(c, apperr.ErrInternal.With("handler wrote no response"), requestID)
		}

		// /healthz 仅正常成功时降噪，取消请求不能被默认 200 掩盖。
		if c.Request.URL.Path == "/healthz" && c.Writer.Status() == http.StatusOK && c.Request.Context().Err() == nil {
			return
		}

		outcome := "ok"
		if c.Request.Context().Err() != nil {
			outcome = "canceled" // 客户端中断：不能把 Gin 默认 200 当正常成功
		} else if st := c.Writer.Status(); st >= 500 {
			outcome = "error"
		} else if st >= 400 {
			outcome = "rejected"
		}

		fields := []zap.Field{
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("route", c.FullPath()),
			zap.Int64("status", int64(c.Writer.Status())),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("client_ip", c.ClientIP()),
			zap.String("outcome", outcome),
		}
		if uid, ok := CurrentUserID(c); ok {
			fields = append(fields, zap.Int64("user_id", uid))
		}
		if len(c.Errors) > 0 {
			// 响应与日志共用归一化后的公开错误结果，便于对照。
			if e, ok := apperr.First(c.Errors.Last().Err); ok {
				fields = append(fields, zap.String("public_error_code", string(e.Code)))
			}
			fields = append(fields, applog.ErrorFields(c.Errors.Last().Err)...)
		}
		for k, v := range applog.Fields(c.Request.Context()) {
			fields = append(fields, zap.Any(k, v))
		}
		switch st := c.Writer.Status(); {
		case outcome == "canceled":
			logger.Warn("request", fields...) // 取消单独降噪
		case st >= 500:
			logger.Error("request", fields...)
		case st >= 400:
			logger.Warn("request", fields...)
		default:
			logger.Info("request", fields...)
		}
	}
}

// newRequestID 生成 16 hex 字符的随机请求标识。
func newRequestID() string {
	b := make([]byte, 8)
	// Go 1.26 的 rand.Read 在熵源故障时终止进程，不会返回可恢复错误。
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ZapRecovery 恢复 handler panic，以 zap 输出堆栈和同源请求标识。
// http.ErrAbortHandler 向 net/http 传播中止语义而非吞掉；已写响应则不再追加第二份 JSON。
func ZapRecovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // 保留 net/http 的中止哨兵语义
			}
			logger.Error("panic",
				zap.String("request_id", RequestIDFrom(c)),
				zap.Any("recover", rec),
				zap.ByteString("stack", debug.Stack()),
			)
			if !c.Writer.Written() {
				response.WriteInternal(c, RequestIDFrom(c))
			}
		}()
		c.Next()
	}
}

// RequireAuth 校验 Bearer access token，通过后将用户 ID 放入上下文供下游读取。
func RequireAuth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, bearerPrefix) {
			response.Fail(c, apperr.ErrUnauthorized.With("missing bearer token"), RequestIDFrom(c))
			c.Abort()
			return
		}
		userID, err := svc.Authenticate(c.Request.Context(), authdto.AuthenticateInput{AccessToken: strings.TrimPrefix(header, bearerPrefix)})
		if err != nil {
			response.Fail(c, err, RequestIDFrom(c))
			c.Abort()
			return
		}
		withUserID(c, userID)
		c.Next()
	}
}
