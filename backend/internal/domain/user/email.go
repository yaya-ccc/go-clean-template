package user

import (
	"regexp"
	"strings"
)

// emailRe 仅做格式校验，不验证邮箱真实存在（MVP 无邮箱验证）。
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// NormalizeEmail 规范化邮箱（去首尾空白、转小写）并报告格式与长度是否合法。
// 无论是否合法都返回规范化后的串：注册语境用 ok 拒绝，登录语境忽略 ok
// 放行非法邮箱、交由查询自然失败，保持坏邮箱与坏密码同为 bad_credentials。
// 邮箱格式与规范化的单一真相源。
func NormalizeEmail(email string) (norm string, ok bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailRe.MatchString(email) || len(email) > 254 { // 254 是邮箱协议长度上限
		return email, false
	}
	return email, true
}
