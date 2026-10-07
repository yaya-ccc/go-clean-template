package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/domain/user/fakes"
	"go-clean-template/backend/internal/kernel/apperr"
	authpres "go-clean-template/backend/internal/presentation/api/auth"
	"go-clean-template/backend/internal/presentation/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type failingRefreshRepo struct {
	*fakes.RefreshTokens
	findCalls, deleteCalls int
}

// Find 模拟系统错误包装凭据错误，检验 Cookie 决策只看最外层语义。
func (r *failingRefreshRepo) Find(context.Context, string) (*user.RefreshToken, error) {
	r.findCalls++
	return nil, apperr.ErrInternal.With("test.refresh.find", apperr.WithCause(apperr.ErrUnauthorized.With("inner credential fact")))
}

// Delete 模拟吊销失败，不得清 Cookie 或误报成功。
func (r *failingRefreshRepo) Delete(context.Context, string) error {
	r.deleteCalls++
	return apperr.ErrInternal.With("test.refresh.delete")
}

func TestSystemFailuresPreserveRefreshCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &failingRefreshRepo{RefreshTokens: fakes.NewRefreshTokens()}
	svc, err := authapp.NewService(fakes.NewUsers(), repo, authapp.NewJWTIssuer("test"), authapp.Config{})
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(middleware.ProcessLog(zap.NewNop()))
	if err := authpres.Register(r.Group("/api/auth"), svc, time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/auth/refresh", "/api/auth/logout"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "existing-credential"})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var body struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 500 || body.Code != "internal_error" || body.RequestID != w.Header().Get("X-Request-Id") {
			t.Fatalf("故障契约不符：%d %s", w.Code, w.Body.String())
		}
		if cookies := w.Result().Cookies(); len(cookies) != 0 {
			t.Fatalf("系统故障不得覆盖或清除 Cookie：%v", cookies)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	if w.Code != 200 || w.Body.String() != `{"status":"ok"}` || repo.deleteCalls != 1 {
		t.Fatalf("无 Cookie 登出应幂等且不访问仓储：%d %s calls=%d", w.Code, w.Body.String(), repo.deleteCalls)
	}
}

func TestInvalidRefreshClearsCookie(t *testing.T) {
	r := newRouter(t)
	for _, raw := range []string{"", "absent-credential"} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
		if raw != "" {
			req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		cookies := w.Result().Cookies()
		if w.Code != 401 || len(cookies) != 1 || cookies[0].Name != "refresh_token" || cookies[0].MaxAge != -1 {
			t.Fatalf("无效凭据应返回 401 并清 Cookie：%d %v", w.Code, cookies)
		}
	}
}

func TestBindingFailureCarriesRequestID(t *testing.T) {
	r := newRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`not json`)))
	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 400 || len(body.RequestID) != 16 || body.RequestID != w.Header().Get("X-Request-Id") {
		t.Fatalf("绑定错误标识不一致：%d %s", w.Code, w.Body.String())
	}
}
