package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-clean-template/backend/internal/application/auth/dto"
	"go-clean-template/backend/internal/domain/user/fakes"
	"go-clean-template/backend/internal/kernel/apperr"

	"golang.org/x/crypto/bcrypt"
)

// TestRegisterValidatesContentWithoutHTTP 防止非 HTTP 调用绕过认证内容规则。
func TestRegisterValidatesContentWithoutHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, email, password, nickname string
	}{
		{"坏邮箱", "not-email", "password123", ""},
		{"短密码", "a@example.com", "short", ""},
		{"超长密码", "a@example.com", strings.Repeat("x", 73), ""},
		{"多字节密码", "a@example.com", strings.Repeat("你", 25), ""},
		{"超长昵称", "a@example.com", "password123", strings.Repeat("你", 65)},
		{"超长默认昵称", strings.Repeat("a", 65) + "@example.com", "password123", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users := fakes.NewUsers()
			svc, err := NewService(users, fakes.NewRefreshTokens(), NewJWTIssuer("test"), Config{
				AccessTTL: time.Minute, RefreshTTL: time.Hour, BCryptCost: bcrypt.MinCost,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.Register(context.Background(), dto.RegisterInput{Email: tc.email, Password: tc.password, Nickname: tc.nickname}); !apperr.CodeIs(err, apperr.ErrInvalidInput) {
				t.Fatalf("内容非法应返回 invalid_input，得到 %v", err)
			}
			if u, err := users.FindByEmail(context.Background(), tc.email); err != nil || u != nil {
				t.Fatalf("校验失败不得写入用户，得到 %v %v", u, err)
			}
		})
	}
}

func TestRegisterNormalizesWithoutHTTP(t *testing.T) {
	svc, err := NewService(fakes.NewUsers(), fakes.NewRefreshTokens(), NewJWTIssuer("test"), Config{
		AccessTTL: time.Minute, RefreshTTL: time.Hour, BCryptCost: bcrypt.MinCost,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Register(context.Background(), dto.RegisterInput{Email: "  Alice@Example.COM ", Password: "password123", Nickname: ""})
	if err != nil {
		t.Fatal(err)
	}
	if res.User.Email != "alice@example.com" || res.User.Nickname != "alice" {
		t.Fatalf("用例应归一化邮箱后派生昵称，得到 %+v", res.User)
	}
	if _, err := svc.Login(context.Background(), dto.LoginInput{Email: " ALICE@EXAMPLE.COM ", Password: "password123"}); err != nil {
		t.Fatalf("登录用例也应归一化邮箱，得到 %v", err)
	}
}

func TestLoginOwnsCredentialSemantics(t *testing.T) {
	svc, _ := newTestService(t, time.Hour)
	if _, err := svc.Login(context.Background(), dto.LoginInput{Email: "absent@example.com", Password: "password123"}); !apperr.CodeIs(err, apperr.ErrBadCredentials) {
		t.Fatalf("登录用例不得把邮箱不存在交给 handler 合码，得到 %v", err)
	}
}

func TestLoginCorruptHashIsProjectError(t *testing.T) {
	users := fakes.NewUsers()
	if _, err := users.Create(context.Background(), "a@example.com", "broken hash", "a"); err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(users, fakes.NewRefreshTokens(), NewJWTIssuer("test"), Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), dto.LoginInput{Email: "a@example.com", Password: "password123"}); !apperr.CodeIs(err, apperr.ErrInternal) {
		t.Fatalf("哈希损坏必须在调用点转换为 internal_error，得到 %v", err)
	}
}
