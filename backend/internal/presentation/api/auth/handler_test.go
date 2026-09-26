package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	"go-clean-template/backend/internal/domain/user/fakes"
	"go-clean-template/backend/internal/presentation/api/auth"

	"github.com/gin-gonic/gin"
)

// newRouter 构造挂载真实 auth 路由与应用服务的测试引擎：
// 仓储用共享内存假件（注册成功路径真实走通），校验行为在 HTTP 边界验证。
func newRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/auth")
	svc, err := authapp.NewService(
		fakes.NewUsers(), fakes.NewRefreshTokens(), authapp.NewJWTIssuer("test-secret"),
		authapp.Config{AccessTTL: time.Minute, RefreshTTL: time.Hour},
	)
	if err != nil {
		t.Fatalf("构造认证服务失败: %v", err)
	}
	if err := auth.Register(api, svc, time.Hour); err != nil {
		t.Fatalf("注册路由失败: %v", err)
	}
	return r
}

// postRegister 发起注册请求，返回状态码与解码后的错误体。
func postRegister(t *testing.T, r *gin.Engine, body string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body))))
	var b map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("响应体解码失败: %v（原始 %s）", err, w.Body.String())
	}
	return w.Code, b
}

// TestRegisterValidationAtEdge 校验注册校验全部在表现层生效：
// 坏邮箱、短密码、超 72 字节密码均 400 invalid_input，且响应不含字段级细节。
func TestRegisterValidationAtEdge(t *testing.T) {
	r := newRouter(t)
	for _, body := range []string{
		`{"email":"not-an-email","password":"password123"}`,                       // 坏邮箱
		`{"email":"a@example.com","password":"short"}`,                            // 短密码
		`{"email":"a@example.com","password":"` + string(make([]byte, 73)) + `"}`, // 超 72 字节（含 NUL 字节）
	} {
		code, b := postRegister(t, r, body)
		if code != http.StatusBadRequest {
			t.Fatalf("应 400，得到 %d（%v）", code, b)
		}
		if b["code"] != "invalid_input" {
			t.Fatalf("错误码应为 invalid_input，得到 %v", b["code"])
		}
		if _, has := b["details"]; has {
			t.Fatal("响应不得携带字段级细节")
		}
	}
}

// TestRegisterNormalizesEmail 校验注册成功路径：表现层归一化邮箱（trim + 小写）后
// 应用层收到小写真值；成功响应 data 直接作响应体，无信封包裹。
func TestRegisterNormalizesEmail(t *testing.T) {
	r := newRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/register",
		bytes.NewReader([]byte(`{"email":"  Alice@Example.COM ","password":"password123"}`))))
	if w.Code != http.StatusOK {
		t.Fatalf("注册应 200，得到 %d %s", w.Code, w.Body.String())
	}
	var b struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("响应解码失败: %v", err)
	}
	if b.Email != "alice@example.com" {
		t.Errorf("邮箱应被归一化，得到 %q", b.Email)
	}
}

// TestLoginBadEmailIndistinguishable 校验登录对坏邮箱不返回 400：
// 与错误密码同为 401 bad_credentials，不可区分。
func TestLoginBadEmailIndistinguishable(t *testing.T) {
	r := newRouter(t)
	for _, body := range []string{
		`{"email":"not-an-email","password":"password123"}`,
		`{"email":"ghost@example.com","password":"password123"}`,
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(body))))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("应 401，得到 %d", w.Code)
		}
		var b map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
			t.Fatalf("解码失败: %v", err)
		}
		if b["code"] != "bad_credentials" {
			t.Fatalf("错误码应为 bad_credentials，得到 %v", b["code"])
		}
	}
}
