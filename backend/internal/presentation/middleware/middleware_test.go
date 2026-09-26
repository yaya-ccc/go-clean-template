package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/kernel/applog"
	"go-clean-template/backend/internal/presentation/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newObservedLogger 返回可观察日志条目的 logger，供中间件行为断言。
func newObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return zap.New(core), logs
}

// fieldVal 从 zapcore.Field 取出实际值；String/Int64 型字段的值不在 Interface 里。
func fieldVal(f zapcore.Field) any {
	switch f.Type {
	case zapcore.StringType:
		return f.String
	case zapcore.Int64Type:
		return f.Integer
	default:
		return f.Interface
	}
}

// fieldsMap 把一条日志的字段展平为 map。
func fieldsMap(entry observer.LoggedEntry) map[string]any {
	m := map[string]any{}
	for _, f := range entry.Context {
		m[f.Key] = fieldVal(f)
	}
	return m
}

func TestProcessLogLevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, nil) })
	r.GET("/bad", func(c *gin.Context) { c.JSON(http.StatusBadRequest, nil) })
	r.GET("/boom", func(c *gin.Context) { c.JSON(http.StatusInternalServerError, nil) })

	for _, path := range []string{"/ok", "/bad", "/boom"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	entries := logs.All()
	if len(entries) != 3 {
		t.Fatalf("应输出 3 条摘要日志，得到 %d", len(entries))
	}
	want := []zapcore.Level{zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel}
	for i, lv := range want {
		if entries[i].Level != lv {
			t.Errorf("第 %d 条日志级别 = %v，want %v", i, entries[i].Level, lv)
		}
	}
}

func TestProcessLogCarriesReservedAndBizFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	r.POST("/answer", func(c *gin.Context) {
		applog.Add(c.Request.Context(), "wrong_count", applog.Int64(2))
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/answer", nil))

	if rid := w.Header().Get("X-Request-Id"); len(rid) != 16 {
		t.Errorf("X-Request-Id 应回显 16 字符请求标识，得到 %q", rid)
	}
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("应输出 1 条摘要日志，得到 %d", len(entries))
	}
	fields := fieldsMap(entries[0])
	for _, key := range []string{"request_id", "method", "route", "status", "latency_ms", "client_ip", "outcome"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("摘要日志缺保留键 %q，实际字段 %v", key, fields)
		}
	}
	if fields["method"] != http.MethodPost || fields["route"] != "/answer" || fields["status"] != int64(200) {
		t.Errorf("保留键取值不符：%v", fields)
	}
	if fields["outcome"] != "ok" {
		t.Errorf("2xx outcome 应为 ok，得到 %v", fields["outcome"])
	}
	if fields["wrong_count"] != int64(2) {
		t.Errorf("业务过程数据应合并进摘要行，得到 %v", fields["wrong_count"])
	}
	if fields["request_id"] != w.Header().Get("X-Request-Id") {
		t.Errorf("日志中 request_id 应与响应头一致：%v vs %q", fields["request_id"], w.Header().Get("X-Request-Id"))
	}
}

func TestProcessLogSkipsHealthyHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, nil) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if n := len(logs.All()); n != 0 {
		t.Errorf("成功的 /healthz 探针不应产生日志，得到 %d 条", n)
	}
}

// TestProcessLogHealthzFailureVisible 校验健康检查失败仍可见。
func TestProcessLogHealthzFailureVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusServiceUnavailable, nil) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	entries := logs.All()
	if len(entries) != 1 || entries[0].Level != zapcore.ErrorLevel {
		t.Fatalf("失败的 /healthz 应输出 Error 摘要，得到 %d 条 %v", len(entries), entries)
	}
}

// TestProcessLogCauseText 校验错误摘要输出链尾原始 cause 原文：
// 驱动错误号与数据值直接可读，排障无需再查数据库。
func TestProcessLogCauseText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	r.POST("/dup", func(c *gin.Context) {
		_ = c.Error(errors.New("Error 1062: Duplicate entry 'alice@example.com' for key 'uq_users_email'"))
		c.JSON(http.StatusConflict, nil)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/dup", nil))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("应输出 1 条摘要，得到 %d 条", len(entries))
	}
	m := fieldsMap(entries[0])
	if cause, _ := m["cause"].(string); !containsAny(cause, "1062", "alice@example.com", "uq_users_email") {
		t.Fatalf("cause 字段应含驱动原文，得到 %q", cause)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestZapRecoveryAndErrorField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger), middleware.ZapRecovery(logger))
	r.GET("/panic", func(c *gin.Context) { panic("boom") })
	r.POST("/login", func(c *gin.Context) {
		_ = c.Error(apperr.ErrCode("bad_credentials").With("auth.login"))
		c.JSON(http.StatusUnauthorized, nil)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("panic 应返回 500，得到 %d", w.Code)
	}

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/login", nil))
	entries := logs.All()
	if len(entries) != 3 { // panic 诊断 + 500 摘要 + 401 摘要（共享 request_id 的明确例外）
		t.Fatalf("应输出 3 条日志，得到 %d", len(entries))
	}
	if entries[0].Message != "panic" {
		t.Errorf("第一条应为 panic 恢复日志，得到 %q", entries[0].Message)
	}
	fields := fieldsMap(entries[2])
	if fields["public_error_code"] != "bad_credentials" {
		t.Errorf("401 摘要应含公开错误码，字段集 %v", fields)
	}
}

// TestZapRecoveryAbortHandlerPanics 校验 http.ErrAbortHandler 保持中止语义
// 而非被恢复吞掉。
func TestZapRecoveryAbortHandlerPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ZapRecovery(logger))
	r.GET("/abort", func(c *gin.Context) { panic(http.ErrAbortHandler) })

	w := httptest.NewRecorder()
	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Errorf("ErrAbortHandler 应向上传播，得到 %v", rec)
		}
		if n := len(logs.All()); n != 0 {
			t.Errorf("中止哨兵不应产生 panic 日志，得到 %d 条", n)
		}
	}()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abort", nil))
}

// TestProcessLogWritesFallbackOnMissingResponse 校验裸签名 handler 漏写响应时，
// ProcessLog 兜底补 500 并在摘要日志留下归因错误。
func TestProcessLogWritesFallbackOnMissingResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	r.GET("/silent", func(c *gin.Context) {}) // 漏写任何响应

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/silent", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("漏写响应应被兜底为 500，得到 %d", w.Code)
	}
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("应输出 1 条摘要日志，得到 %d 条", len(entries))
	}
	if fields := fieldsMap(entries[0]); fields["public_error_code"] != "internal_error" {
		t.Errorf("摘要应含归因公开码 internal_error，字段集 %v", fields)
	}
}
