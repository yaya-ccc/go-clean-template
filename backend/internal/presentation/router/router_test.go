package router_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authapp "go-clean-template/backend/internal/application/auth"
	userapp "go-clean-template/backend/internal/application/user"
	"go-clean-template/backend/internal/domain/user/fakes"
	"go-clean-template/backend/internal/presentation/router"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

func TestFrameworkAndAuthErrorsUseUnifiedExit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 已关闭的真实 sql.DB 确定性触发探针失败，不依赖本机数据库与网络。
	db, err := sql.Open("mysql", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	users := fakes.NewUsers()
	issuer := authapp.NewJWTIssuer("test")
	authSvc, err := authapp.NewService(users, fakes.NewRefreshTokens(), issuer, authapp.Config{})
	if err != nil {
		t.Fatal(err)
	}
	userSvc, err := userapp.NewService(users)
	if err != nil {
		t.Fatal(err)
	}
	r, err := router.New(router.Deps{DB: db, AuthSvc: authSvc, UserSvc: userSvc})
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Issue(99, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, token, code string
		status                          int
	}{
		{"路径", http.MethodGet, "/missing", "", "route_not_found", 404},
		{"方法", http.MethodGet, "/api/auth/register", "", "method_not_allowed", 405},
		{"探针", http.MethodGet, "/healthz", "", "internal_error", 500},
		{"无凭据", http.MethodGet, "/api/users/me", "", "unauthorized", 401},
		{"用户不存在", http.MethodGet, "/api/users/me", token, "unauthorized", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			var body struct {
				Code      string `json:"code"`
				RequestID string `json:"request_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Code != tc.code || len(body.RequestID) != 16 || body.RequestID != w.Header().Get("X-Request-Id") {
				t.Fatalf("统一出口不符：%d %s", w.Code, w.Body.String())
			}
		})
	}
}
