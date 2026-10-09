// Package auth 编排注册、登录、令牌签发与刷新的用例。
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-clean-template/backend/internal/application/auth/dto"
	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/kernel/applog"

	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Secret     string        // JWT HMAC 签名密钥
	AccessTTL  time.Duration // access token 有效期
	RefreshTTL time.Duration // refresh token 有效期
	BCryptCost int           // bcrypt cost；0 表示用库默认
}

// dummyHash 是登录未命中路径的固定 bcrypt 哈希：比对注定失败但代价真实，
// 用于拉平与命中路径的响应时间差（防用户枚举的时间侧信道)）。
// 由固定口令派生，替换轮换不影响安全性质（两条路径只要求同代价，不要求同哈希）。
const dummyHash = "$2a$10$.4FqfS7dL8c5I6sRiVuwZO.RzI7AvifYi2bCz8joztUkWzmmPTOcW"

// Service 是认证用例的编排入口。
type Service struct {
	users   user.Repository
	refresh user.RefreshTokenRepository
	tokens  TokenIssuer
	cfg     Config
}

// TokenIssuer 抽象 access token 的签发与校验，避免应用层依赖具体 JWT 库。
type TokenIssuer interface {
	// Issue 返回给 userID 签发的已签名 access token 字符串。
	Issue(userID int64, ttl time.Duration) (string, error)
	// Parse 校验 token 签名与有效期，返回 userID；
	// 凭据无效返回 unauthorized 语义，系统故障保留内部错误。
	Parse(token string) (int64, error)
}

// NewService 构造认证服务；全部协作为必填，任一为 nil 视为组装错误直接返回错误，
// 不提供降级运行。
func NewService(
	users user.Repository,
	refresh user.RefreshTokenRepository,
	tokens TokenIssuer,
	cfg Config,
) (*Service, error) {
	if users == nil {
		return nil, apperr.ErrInternal.With("auth.NewService: users repository must not be nil")
	}
	if refresh == nil {
		return nil, apperr.ErrInternal.With("auth.NewService: refresh repository must not be nil")
	}
	if tokens == nil {
		return nil, apperr.ErrInternal.With("auth.NewService: token issuer must not be nil")
	}
	return &Service{users: users, refresh: refresh, tokens: tokens, cfg: cfg}, nil
}

// Register 用邮箱密码注册新用户，成功即视为登录，返回用户视图与令牌对。
// 邮箱唯一性由唯一索引在写入时判定，冲突经仓储边界分类直接产出 email_taken。
// 注册不包事务：用户创建成功即既成事实，其后签发失败只上抛错误、不回滚；
// 用户重试注册会得到 email_taken，转登录即可。
func (s *Service) Register(ctx context.Context, in dto.RegisterInput) (dto.AuthResult, error) {
	email, ok := user.NormalizeEmail(in.Email)
	if !ok {
		return dto.AuthResult{}, apperr.ErrInvalidInput.With("auth.register.email_invalid")
	}
	if err := user.ValidatePassword(in.Password); err != nil {
		return dto.AuthResult{}, err
	}
	nickname := in.Nickname
	if nickname == "" {
		nickname = emailPrefix(email)
	}
	if err := user.ValidateNickname(nickname); err != nil {
		return dto.AuthResult{}, err
	}
	hash, err := hashPassword(in.Password, s.cfg.BCryptCost)
	if err != nil {
		return dto.AuthResult{}, err
	}
	created, err := s.users.Create(ctx, email, hash, nickname)
	if err != nil {
		return dto.AuthResult{}, err
	}
	pair, err := s.issuePair(ctx, created.ID)
	if err != nil {
		return dto.AuthResult{}, err
	}
	return dto.AuthResult{User: created, Tokens: pair}, nil
}

// Login 校验邮箱密码，返回用户视图与令牌对。
// 邮箱不存在与密码错误在用例内同产 bad_credentials，不依赖 HTTP 入口合码。
func (s *Service) Login(ctx context.Context, in dto.LoginInput) (dto.AuthResult, error) {
	// 非法邮箱只归一化不拒绝，查询自然未命中，保持与坏密码不可区分。
	email, _ := user.NormalizeEmail(in.Email)
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return dto.AuthResult{}, err
	}
	if u == nil {
		// 与命中用户走同代价的 bcrypt 比对拉平响应时间，堵住「查无此人立即返回」
		// 的时间侧信道（用户枚举)）。
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(in.Password))
		return dto.AuthResult{}, apperr.ErrBadCredentials.With("login email not found")
	}
	if err := verifyPassword(u.PasswordHash, in.Password); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return dto.AuthResult{}, apperr.ErrBadCredentials.With("login password mismatch")
		}
		// 库中哈希损坏、不受支持的版本/代价等：在第三方调用边界转换，
		// 不裸抛，也不谎称用户输错密码。
		return dto.AuthResult{}, apperr.ErrInternal.With("auth.login.verify_password", apperr.WithCause(err))
	}
	pair, err := s.issuePair(ctx, u.ID)
	if err != nil {
		return dto.AuthResult{}, err
	}
	return dto.AuthResult{User: u, Tokens: pair}, nil
}

// Refresh 用刷新凭据换新令牌对；未命中或过期在用例内统一为 unauthorized。
// 过期凭据的惰性清理失败只记日志，不改变主结果。
func (s *Service) Refresh(ctx context.Context, in dto.RefreshInput) (user.TokenPair, error) {
	rawToken := in.RefreshToken
	if rawToken == "" {
		return user.TokenPair{}, apperr.ErrUnauthorized.With("empty refresh token")
	}
	tokenHash := hashToken(rawToken)
	stored, err := s.refresh.Find(ctx, tokenHash)
	if err != nil {
		return user.TokenPair{}, err
	}
	if stored == nil {
		return user.TokenPair{}, apperr.ErrUnauthorized.With("refresh token not found")
	}
	if time.Now().After(stored.ExpiresAt) {
		if derr := s.refresh.Delete(ctx, tokenHash); derr != nil { // 惰性清理：附带诊断事件
			applog.Add(ctx, "expired_cleanup_failed", applog.Bool(true))
			applog.Add(ctx, "expired_cleanup_error", applog.Err(derr))
		}
		return user.TokenPair{}, apperr.ErrUnauthorized.With("refresh token expired")
	}
	return s.issuePair(ctx, stored.UserID)
}

// Authenticate 校验 access token 还原用户 ID，供需要登录的接口复用。
// 错误分类由 TokenIssuer 契约保证：凭据无效 unauthorized，内部故障原样上抛
// ；此处不重新包装已封装好的错误。
func (s *Service) Authenticate(_ context.Context, in dto.AuthenticateInput) (int64, error) {
	return s.tokens.Parse(in.AccessToken)
}

// Logout 吊销刷新凭据；凭据本就不存在时也视为成功（幂等），存储故障原样返回。
func (s *Service) Logout(ctx context.Context, in dto.LogoutInput) error {
	return s.refresh.Delete(ctx, hashToken(in.RefreshToken))
}

// issuePair 同时签发 access 与 refresh；refresh 明文只出现一次，库中只存哈希。
// ponytail: refresh 不做轮换（刷新不删旧凭据，新旧并存直至过期），
// 被盗检测是升级路径：加 rotation（刷新时删旧）即可，表结构无需变。
func (s *Service) issuePair(ctx context.Context, userID int64) (user.TokenPair, error) {
	access, err := s.tokens.Issue(userID, s.cfg.AccessTTL)
	if err != nil {
		return user.TokenPair{}, err
	}
	raw := randomToken()
	err = s.refresh.Save(ctx, user.RefreshToken{
		TokenHash: hashToken(raw),
		UserID:    userID,
		ExpiresAt: time.Now().Add(s.cfg.RefreshTTL),
		CreatedAt: time.Now(),
	})
	if err != nil {
		return user.TokenPair{}, err
	}
	return user.TokenPair{Access: access, Refresh: raw}, nil
}

// emailPrefix 取邮箱 @ 前部分作为默认昵称；无 @ 时以原串兜底而非 panic——
// '@' 存在这一不变量由注册用例的邮箱校验保证。
func emailPrefix(email string) string {
	if prefix, _, ok := strings.Cut(email, "@"); ok {
		return prefix
	}
	return email
}

// hashPassword 用 bcrypt 生成密码哈希；cost 为 0 时使用库默认值。
// 输入长度已由注册用例调用领域规则校验，此处失败即为内部故障。
func hashPassword(password string, cost int) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", apperr.ErrInternal.With("bcrypt hash", apperr.WithCause(err))
	}
	return string(h), nil
}

// verifyPassword 比对密码与哈希，返回 error 以区分「密码不匹配」（ErrMismatchedHashAndPassword）
// 与「库中哈希损坏/不受支持」（内部故障）；登录语境对超长输入同样走比对失败。
func verifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// hashToken 计算刷新令牌的存储哈希（SHA-256 十六进制）。
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// randomToken 生成 256 位随机刷新令牌的十六进制表示。
// Go 1.26 起 crypto/rand.Read 不返回普通错误（不可恢复失败终止进程），
// panic 分支仅防御性保留，不是生产容错承诺。
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand 失败: %v", err)) // 系统熵源不可用时无法安全签发凭据
	}
	return hex.EncodeToString(b)
}
