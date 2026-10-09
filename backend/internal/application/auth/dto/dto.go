// Package dto 定义认证用例的输入与多值产出，不携带 HTTP 序列化标签。
package dto

import "go-clean-template/backend/internal/domain/user"

// AuthResult 收拢注册与登录的两项产出；刷新只有令牌对，不另造包装类型。
type AuthResult struct {
	User   *user.User
	Tokens user.TokenPair
}

// RegisterInput 是注册用例输入；内容校验与邮箱归一化由用例保证，不依赖 HTTP 入口。
type RegisterInput struct {
	Email    string
	Password string
	Nickname string
}

// LoginInput 保留原始邮箱与密码，由登录语境统一处理非法凭据。
type LoginInput struct {
	Email    string
	Password string
}

// RefreshInput 携带刷新凭据明文，持久化前须由用例散列。
type RefreshInput struct {
	RefreshToken string
}

// LogoutInput 携带待吊销的刷新凭据，不包含任何 HTTP Cookie 细节。
type LogoutInput struct {
	RefreshToken string
}

// AuthenticateInput 携带待校验的 access token，不约束其 HTTP 来源。
type AuthenticateInput struct {
	AccessToken string
}
