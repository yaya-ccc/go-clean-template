package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go-clean-template/backend/internal/domain/user/fakes"
	"go-clean-template/backend/internal/kernel/apperr"
)

// newTestService 用共享仓储假件构造认证服务。
func newTestService(t *testing.T, refreshTTL time.Duration) (*Service, *fakes.RefreshTokens) {
	t.Helper()
	svc, err := NewService(fakes.NewUsers(), fakes.NewRefreshTokens(), NewJWTIssuer("test-secret"), Config{
		AccessTTL:  time.Minute,
		RefreshTTL: refreshTTL,
	})
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}
	return svc, fakes.NewRefreshTokens()
}

func TestRegisterLoginRefreshLogout(t *testing.T) {
	ctx := context.Background()
	users := fakes.NewUsers()
	refresh := fakes.NewRefreshTokens()
	svc, err := NewService(users, refresh, NewJWTIssuer("test-secret"), Config{
		AccessTTL: time.Minute, RefreshTTL: 30 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}

	// 注册成功：默认昵称取邮箱前缀、令牌对可解析
	res, err := svc.Register(ctx, "alice@example.com", "password123", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if res.User.Nickname != "alice" {
		t.Errorf("默认昵称应为邮箱前缀，得到 %s", res.User.Nickname)
	}
	if len(res.Tokens.Access) == 0 || len(res.Tokens.Refresh) != 64 {
		t.Errorf("令牌形态异常: access=%d refresh=%d", len(res.Tokens.Access), len(res.Tokens.Refresh))
	}
	if id, err := svc.Authenticate(res.Tokens.Access); err != nil || id != res.User.ID {
		t.Errorf("access token 无法还原用户: id=%d err=%v", id, err)
	}

	// 重复注册同邮箱冲突
	if _, err := svc.Register(ctx, "alice@example.com", "password123", ""); !errors.Is(err, apperr.ErrEmailTaken) {
		t.Fatalf("重复邮箱应 ErrEmailTaken，得到 %v", err)
	}
	// 登录：错误密码产出 bad_credentials；不存在邮箱产出事实码 user_not_found，
	// 由表现层合码为 bad_credentials，应用层二者不同码便于日志归因。
	if _, err := svc.Login(ctx, "alice@example.com", "wrongpass1"); !errors.Is(err, apperr.ErrBadCredentials) {
		t.Fatalf("错误密码应 ErrBadCredentials，得到 %v", err)
	}
	if _, err := svc.Login(ctx, "nobody@example.com", "password123"); !errors.Is(err, apperr.ErrUserNotFound) {
		t.Fatalf("不存在邮箱应 ErrUserNotFound，得到 %v", err)
	}
	login, err := svc.Login(ctx, "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if _, err := svc.Authenticate(login.Tokens.Access); err != nil {
		t.Errorf("登录签发的 access 无效: %v", err)
	}

	// 刷新：旧 refresh 换新对；登出后同 token 再刷产出事实码（凭据已吊销），
	// 表现层合码为 unauthorized。
	pair, err := svc.Refresh(ctx, login.Tokens.Refresh)
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}
	if err := svc.Logout(ctx, pair.Refresh); err != nil {
		t.Fatalf("登出失败: %v", err)
	}
	if _, err := svc.Refresh(ctx, pair.Refresh); !errors.Is(err, apperr.ErrRefreshTokenNotFound) {
		t.Fatalf("登出后刷新应 ErrRefreshTokenNotFound，得到 %v", err)
	}
}

// TestRegisterUnclassifiedConflict 验证白名单外写入冲突透传 database_error，
// 不被误报为邮箱占用。
func TestRegisterUnclassifiedConflict(t *testing.T) {
	users := &fakes.UnknownDupUsers{Users: fakes.NewUsers()}
	refresh := fakes.NewRefreshTokens()
	svc, err := NewService(users, refresh, NewJWTIssuer("test-secret"), Config{
		AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}

	if _, err := svc.Register(context.Background(), "x@example.com", "password123", ""); !errors.Is(err, apperr.ErrDatabase) {
		t.Fatalf("未识别冲突应为 database_error，得到 %v", err)
	}
	if errors.Is(err, apperr.ErrEmailTaken) {
		t.Fatal("未识别冲突不得误报 email_taken")
	}
}

// TestRegisterIssueFailure 验证签发失败不回滚用户创建：
// 错误原样上抛，用户已存在，重试注册得到 email_taken。
func TestRegisterIssueFailure(t *testing.T) {
	users := fakes.NewUsers()
	saveErr := fmt.Errorf("db write failed")
	// Save 恒失败的 refresh 仓储：注册中签发失败，但用户创建不随之撤销。
	failing := fakes.NewFailingRefreshTokens(saveErr)
	svc, err := NewService(users, failing, NewJWTIssuer("test-secret"), Config{
		AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}

	ctx := context.Background()
	if _, err := svc.Register(ctx, "partial@example.com", "password123", ""); err == nil || errors.Is(err, apperr.ErrEmailTaken) {
		t.Fatalf("签发失败应上抛原始错误，得到 %v", err)
	}
	// 用户已落库：同邮箱重试注册命中唯一冲突。
	if _, err := svc.Register(ctx, "partial@example.com", "password123", ""); !errors.Is(err, apperr.ErrEmailTaken) {
		t.Fatalf("重试注册应 ErrEmailTaken，得到 %v", err)
	}
	// 换正常 refresh 仓储（共享同一 users 假件）：签发失败后的用户可用原密码登录。
	okSvc, err := NewService(users, fakes.NewRefreshTokens(), NewJWTIssuer("test-secret"), Config{
		AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}
	if login, err := okSvc.Login(ctx, "partial@example.com", "password123"); err != nil || login.User.ID == 0 {
		t.Fatalf("签发失败后的用户应可登录，得到 %v %v", login.User, err)
	}
}

func TestRefreshExpired(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t, -time.Second) // TTL 为负：立即过期

	reg, err := svc.Register(ctx, "bob@example.com", "password123", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if _, err := svc.Refresh(ctx, reg.Tokens.Refresh); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("过期刷新应 ErrUnauthorized，得到 %v", err)
	}
}

func TestJWTRejectsWrongKeyAndAlg(t *testing.T) {
	// 用另一密钥签发的 token 必须被拒
	forged := NewJWTIssuer("other-secret")
	svc, _ := newTestService(t, time.Hour)
	tok, err := forged.Issue(1, time.Minute)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if _, err := svc.Authenticate(tok); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("伪造 token 应 ErrUnauthorized，得到 %v", err)
	}
}

// TestJWTParseMalformedClaims 验证 sub 缺失/非法与 exp 缺失受控拒绝，不 panic。
func TestJWTParseMalformedClaims(t *testing.T) {
	issuer := NewJWTIssuer("test-secret")
	cases := []struct {
		name   string
		claims map[string]any
	}{
		{"缺 sub", map[string]any{"exp": time.Now().Add(time.Minute).Unix()}},
		{"sub 非数字", map[string]any{"sub": "abc", "exp": time.Now().Add(time.Minute).Unix()}},
		{"sub 非整数", map[string]any{"sub": 1.5, "exp": time.Now().Add(time.Minute).Unix()}},
		{"sub 越界", map[string]any{"sub": -1, "exp": time.Now().Add(time.Minute).Unix()}},
		{"缺 exp", map[string]any{"sub": float64(1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok, err := issuer.signClaims(tc.claims)
			if err != nil {
				t.Fatalf("签发失败: %v", err)
			}
			if _, err := issuer.Parse(tok); !errors.Is(err, apperr.ErrUnauthorized) {
				t.Fatalf("畸形声明应受控 unauthorized，得到 %v", err)
			}
		})
	}
	// 空 token
	if _, err := issuer.Parse(""); !errors.Is(err, apperr.ErrUnauthorized) {
		t.Fatalf("空 token 应 unauthorized，得到 %v", err)
	}
}

// signClaims 用测试密钥签发任意声明，供畸形声明用例构造 token。
func (i *JWTIssuer) signClaims(claims map[string]any) (string, error) {
	return jwtSign(i.secret, claims)
}

// TestNewServiceRejectsNilDeps 验证任一协作为 nil 时构造直接报错，不产出可降级运行的服务实例。
func TestNewServiceRejectsNilDeps(t *testing.T) {
	users := fakes.NewUsers()
	refresh := fakes.NewRefreshTokens()
	cfg := Config{AccessTTL: time.Minute, RefreshTTL: time.Hour}
	cases := []struct {
		name string
		svc  func() (*Service, error)
	}{
		{"users", func() (*Service, error) { return NewService(nil, refresh, NewJWTIssuer("s"), cfg) }},
		{"refresh", func() (*Service, error) { return NewService(users, nil, NewJWTIssuer("s"), cfg) }},
		{"tokens", func() (*Service, error) { return NewService(users, refresh, nil, cfg) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := tc.svc()
			if err == nil || svc != nil {
				t.Fatalf("nil %s 应构造报错，得到 svc=%v err=%v", tc.name, svc, err)
			}
		})
	}
}
