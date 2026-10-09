package user

import (
	"strings"
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
)

func TestPasswordByteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		password string
		valid    bool
	}{
		{strings.Repeat("x", 7), false},
		{strings.Repeat("x", 8), true},
		{strings.Repeat("x", 72), true},
		{strings.Repeat("x", 73), false},
		{strings.Repeat("你", 24), true},
		{strings.Repeat("你", 25), false},
	} {
		err := ValidatePassword(tc.password)
		if (err == nil) != tc.valid || (!tc.valid && !apperr.CodeIs(err, apperr.ErrInvalidInput)) {
			t.Fatalf("密码 %d 字节的规则不符：%v", len(tc.password), err)
		}
	}
}

func TestNicknameRuneBoundaries(t *testing.T) {
	for _, n := range []int{0, 64, 65} {
		err := ValidateNickname(strings.Repeat("你", n))
		if (err == nil) != (n <= 64) {
			t.Fatalf("昵称 %d 字符的规则不符：%v", n, err)
		}
	}
}
