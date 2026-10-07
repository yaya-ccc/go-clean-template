package user

import (
	"unicode/utf8"

	"go-clean-template/backend/internal/kernel/apperr"
)

const (
	minPasswordBytes = 8
	maxPasswordBytes = 72 // bcrypt 输入上限按字节计数，按字符限制会放过多字节密码。
	maxNicknameRunes = 64
)

// ValidatePassword 保证注册密码满足产品下限与 bcrypt 输入上限，供任意用例入口复用。
func ValidatePassword(password string) error {
	if len(password) < minPasswordBytes {
		return apperr.ErrInvalidInput.With("user.password.too_short")
	}
	if len(password) > maxPasswordBytes {
		return apperr.ErrInvalidInput.With("user.password.too_long")
	}
	return nil
}

// ValidateNickname 按字符限制昵称；空昵称由注册用例派生默认值，不在此拒绝。
func ValidateNickname(nickname string) error {
	if utf8.RuneCountInString(nickname) > maxNicknameRunes {
		return apperr.ErrInvalidInput.With("user.nickname.too_long")
	}
	return nil
}
