package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/middleware"

	"github.com/gin-gonic/gin"
)

type shapeRequest struct {
	Value string `json:"value" binding:"required"`
}

// Validate 用哑规则验证适配器确实调用 DTO 的协议形状门。
func (r *shapeRequest) Validate() error {
	if r.Value == "invalid-shape" {
		return apperr.ErrInvalidInput.With("shape rejected")
	}
	return nil
}

func TestBindAndHandleContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger))
	calls := 0
	r.POST("/body", middleware.Bind[shapeRequest](func(c *gin.Context, req *shapeRequest) (any, error) {
		calls++
		return struct {
			Value string `json:"value"`
		}{Value: req.Value}, nil
	}))
	r.GET("/ack", middleware.Handle(func(c *gin.Context) (any, error) { return nil, nil }))
	r.GET("/error", middleware.Handle(func(c *gin.Context) (any, error) {
		return "ignored success", apperr.ErrInternal.With("failed", apperr.WithCause(errors.New("secret")))
	}))
	for _, body := range []string{`not json`, `{}`, `{"value":"invalid-shape"}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/body", bytes.NewBufferString(body)))
		if w.Code != 400 || !bytes.Contains(w.Body.Bytes(), []byte(`"code":"invalid_input"`)) {
			t.Fatalf("协议绑定或校验应返回 invalid_input：%d %s", w.Code, w.Body.String())
		}
	}
	if calls != 0 {
		t.Fatalf("非法请求不得进入 handler，调用了 %d 次", calls)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/body", bytes.NewBufferString(`{"value":"ok"}`)))
	if calls != 1 || w.Code != 200 || w.Body.String() != `{"value":"ok"}` {
		t.Fatalf("类型化绑定不符：calls=%d %d %s", calls, w.Code, w.Body.String())
	}
	for path, body := range map[string]string{"/ack": `{"status":"ok"}`, "/error": ""} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if path == "/ack" && (w.Code != 200 || w.Body.String() != body) {
			t.Fatalf("nil 成功应渲染 Ack：%d %s", w.Code, w.Body.String())
		}
		if path == "/error" && (w.Code != 500 || bytes.Contains(w.Body.Bytes(), []byte("ignored")) || bytes.Contains(w.Body.Bytes(), []byte("secret"))) {
			t.Fatalf("错误应优先且不泄漏诊断：%d %s", w.Code, w.Body.String())
		}
	}
}

func TestRequestIDIsSharedByEveryErrorPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger), middleware.ZapRecovery(logger))
	r.POST("/bind", middleware.Bind[shapeRequest](func(c *gin.Context, req *shapeRequest) (any, error) { return nil, nil }))
	r.GET("/fail", middleware.Handle(func(c *gin.Context) (any, error) { return nil, errors.New("unknown failure") }))
	r.GET("/panic", func(c *gin.Context) { panic("boom") })
	r.GET("/silent", func(c *gin.Context) {})
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/bind", `{}`},
		{http.MethodGet, "/fail", ""},
		{http.MethodGet, "/panic", ""},
		{http.MethodGet, "/silent", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			logs.TakeAll()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body)))
			var body struct {
				RequestID string `json:"request_id"`
				Code      string `json:"code"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			rid := w.Header().Get("X-Request-Id")
			if len(rid) != 16 || body.RequestID != rid {
				t.Fatalf("错误体和响应头标识不同源：%s %s", body.RequestID, rid)
			}
			entries := logs.All()
			for _, entry := range entries {
				if fieldsMap(entry)["request_id"] != rid {
					t.Fatalf("诊断或摘要标识不同源：%v", fieldsMap(entry))
				}
			}
			if len(entries) == 0 || fieldsMap(entries[len(entries)-1])["public_error_code"] != body.Code {
				t.Fatalf("摘要公开码与响应不一致：%v", entries)
			}
		})
	}
}

func TestCancellationBypassesRenderingAndFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, logs := newObservedLogger()
	r := gin.New()
	r.Use(middleware.ProcessLog(logger), middleware.ZapRecovery(logger))
	r.GET("/late", middleware.Handle(func(c *gin.Context) (any, error) {
		ctx, cancel := context.WithCancel(c.Request.Context())
		cancel()
		c.Request = c.Request.WithContext(ctx)
		return nil, apperr.ErrInternal.With("operation canceled", apperr.WithCause(context.Canceled))
	}))
	r.GET("/healthz", func(c *gin.Context) {})
	for _, path := range []string{"/late", "/healthz"} {
		logs.TakeAll()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if path == "/healthz" {
			ctx, cancel := context.WithCancel(req.Context())
			cancel()
			req = req.WithContext(ctx)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Body.Len() != 0 {
			t.Fatalf("取消出口及漏响应兜底都不应写 JSON：%s", w.Body.String())
		}
		entries := logs.All()
		if len(entries) != 1 || fieldsMap(entries[0])["outcome"] != "canceled" {
			t.Fatalf("取消请求应保留摘要，不能被 healthz 降噪吞掉：%v", entries)
		}
	}
}
