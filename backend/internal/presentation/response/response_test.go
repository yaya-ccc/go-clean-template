package response_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
)

const requestID = "0123456789abcdef"

type errBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func TestFailContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"输入", apperr.ErrInvalidInput.With("shape"), 400, "invalid_input", "请求参数无效"},
		{"凭据", apperr.ErrBadCredentials.With("no user"), 401, "bad_credentials", "邮箱或密码错误"},
		{"身份", apperr.ErrUnauthorized.With("deleted"), 401, "unauthorized", "登录已过期，请重新登录"},
		{"邮箱占用", apperr.ErrEmailTaken.With("duplicate"), 409, "email_taken", "邮箱已被注册"},
		{"路径", apperr.ErrRouteNotFound.With("route"), 404, "route_not_found", "接口不存在"},
		{"方法", apperr.ErrMethodNotAllowed.With("method"), 405, "method_not_allowed", "请求方法不允许"},
		{"系统", apperr.ErrInternal.With("query", apperr.WithCause(errors.New("secret driver text"))), 500, "internal_error", "服务器内部错误"},
		{"裸错误", errors.New("secret driver text"), 500, "internal_error", "服务器内部错误"},
		{"未登记码", apperr.ErrCode("unknown").With("secret op"), 500, "internal_error", "服务器内部错误"},
		{"废弃数据库码", apperr.ErrCode("database_error").With("query"), 500, "internal_error", "服务器内部错误"},
		{"废弃超时码", apperr.ErrCode("operation_timeout").With("slow"), 500, "internal_error", "服务器内部错误"},
		{"废弃不可用码", apperr.ErrCode("service_unavailable").With("down"), 500, "internal_error", "服务器内部错误"},
		{"外层语义取胜", apperr.ErrInternal.With("system", apperr.WithCause(apperr.ErrEmailTaken.With("duplicate"))), 500, "internal_error", "服务器内部错误"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", func(c *gin.Context) { response.Fail(c, tc.err, requestID) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			var b errBody
			if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || b.Code != tc.code || b.Message != tc.message || b.RequestID != requestID {
				t.Fatalf("错误契约不符：%d %+v", w.Code, b)
			}
			if bytes.Contains(w.Body.Bytes(), []byte("secret")) {
				t.Fatalf("响应泄漏内部文本：%s", w.Body.String())
			}
		})
	}
}

func TestFallbackPreservesCauseAndPublicLogCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, original := range []error{errors.New("driver failed"), apperr.ErrCode("unknown").With("unregistered")} {
		r := gin.New()
		r.GET("/", func(c *gin.Context) {
			response.Fail(c, original, requestID)
			logged := c.Errors.Last().Err
			if !apperr.CodeIs(logged, apperr.ErrInternal) || !errors.Is(logged, original) {
				t.Fatalf("日志当前码应与响应一致并保留原因链，得到 %v", logged)
			}
		})
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}
}

func TestFailNilAndWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/nil", func(c *gin.Context) { response.Fail(c, nil, requestID) })
	r.GET("/written", func(c *gin.Context) {
		response.Ack(c)
		response.Fail(c, errors.New("late failure"), requestID)
		response.OK(c, "second body")
	})
	for path, body := range map[string]string{"/nil": "", "/written": `{"status":"ok"}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 200 || w.Body.String() != body {
			t.Fatalf("响应守卫不符：%s %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestCanceledRequestWritesNothing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, write := range []func(*gin.Context){
		func(c *gin.Context) {
			response.Fail(c, apperr.ErrInternal.With("canceled", apperr.WithCause(context.Canceled)), requestID)
		},
		func(c *gin.Context) { response.WriteInternal(c, requestID) },
		response.Ack,
		func(c *gin.Context) { response.OK(c, "data") },
	} {
		r := gin.New()
		r.GET("/", write)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
		if w.Body.Len() != 0 {
			t.Fatalf("请求取消后不得写 JSON，得到 %s", w.Body.String())
		}
	}
}

func TestDependencyCancellationIsStillSystemFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		response.Fail(c, apperr.ErrInternal.With("dependency canceled", apperr.WithCause(context.Canceled)), requestID)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 500 || !bytes.Contains(w.Body.Bytes(), []byte(`"code":"internal_error"`)) {
		t.Fatalf("请求有效时不得因内层 canceled 丢弃响应，得到 %d %s", w.Code, w.Body.String())
	}
}

func TestAckContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/logout", response.Ack)
	r.GET("/healthz", response.Ack)
	for _, tc := range []struct{ method, path string }{{http.MethodPost, "/logout"}, {http.MethodGet, "/healthz"}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != 200 || w.Body.String() != `{"status":"ok"}` {
			t.Fatalf("Ack 契约不符：%d %s", w.Code, w.Body.String())
		}
	}
}
