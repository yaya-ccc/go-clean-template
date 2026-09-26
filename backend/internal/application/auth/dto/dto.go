// Package dto 集中认证模块应用层的 DTO：用例多于一个成功产出时收拢的命名类型。
package dto

import "go-clean-template/backend/internal/domain/user"

// AuthResult 是注册与登录共用的认证产出 DTO：成功即视为登录，
// 含用户视图与令牌对；刷新用例不产出用户视图，直接返回令牌对。
type AuthResult struct {
	User   *user.User
	Tokens user.TokenPair
}
