package auth

import (
	"time"

	"go-clean-template/backend/internal/kernel/apperr"

	"github.com/golang-jwt/jwt/v5"
)

// JWTIssuer 是 TokenIssuer 的 JWT HMAC 实现，属应用层的令牌适配。
// ponytail: 密钥与算法写死 HS256，需非对称或多密钥轮换时再抽象。
type JWTIssuer struct {
	secret []byte
}

// NewJWTIssuer 用 HMAC 密钥构造签发器；密钥由 main 从配置注入。
func NewJWTIssuer(secret string) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret)}
}

// Issue 给 userID 签发指定有效期的 HS256 JWT；签名失败即转换为 apperr。
func (i *JWTIssuer) Issue(userID int64, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", apperr.ErrInternal.With("sign jwt", apperr.WithCause(err))
	}
	return signed, nil
}

// Parse 校验签名与有效期，从 sub 取回 userID。
// 凭据类失败（签名、格式、过期、算法、声明缺失/非法）返回 unauthorized；
// 普通日志不得记录完整 token、签名或解析后的敏感 claims。
func (i *JWTIssuer) Parse(token string) (int64, error) {
	var claims jwt.MapClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return i.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), // 仅接受 HS256，拒绝算法替换攻击
		jwt.WithExpirationRequired()) // 项目签发的访问令牌必须携带 exp
	if err != nil {
		return 0, apperr.ErrUnauthorized.With("parse jwt")
	}
	id, err := parseSubject(claims)
	if err != nil {
		return 0, apperr.ErrUnauthorized.With("parse sub")
	}
	return id, nil
}

// parseSubject 安全读取数字 sub 声明：检查存在性、类型、整数性与合法 ID，
// 不做直接类型断言。
func parseSubject(claims jwt.MapClaims) (int64, error) {
	raw, ok := claims["sub"]
	if !ok {
		return 0, apperr.ErrUnauthorized.With("missing sub")
	}
	sub, ok := raw.(float64) // encoding/json 数字统一解码为 float64
	if !ok || sub != float64(int64(sub)) || sub < 1 {
		return 0, apperr.ErrUnauthorized.With("invalid sub")
	}
	return int64(sub), nil
}
