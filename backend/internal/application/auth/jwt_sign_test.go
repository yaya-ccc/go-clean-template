package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// jwtSign 用指定密钥签发任意声明，仅测试使用：构造畸形 token 验证 Parse 的受控拒绝。
func jwtSign(secret []byte, claims map[string]any) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims)).SignedString(secret)
}
