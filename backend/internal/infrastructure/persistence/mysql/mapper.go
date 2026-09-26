package mysql

import (
	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/infrastructure/persistence/mysql/po"
)

// toDomainUser 把 users 表行形状转为领域模型；两侧字段语义一致（UTC），逐字段直拷。
func toDomainUser(p po.User) user.User {
	return user.User{
		ID:           p.ID,
		Email:        p.Email,
		PasswordHash: p.PasswordHash,
		Nickname:     p.Nickname,
		CreatedAt:    p.CreatedAt,
	}
}

// toDomainRefreshToken 把 refresh_tokens 表行形状转为领域模型。
func toDomainRefreshToken(p po.RefreshToken) user.RefreshToken {
	return user.RefreshToken{
		TokenHash: p.TokenHash,
		UserID:    p.UserID,
		ExpiresAt: p.ExpiresAt,
		CreatedAt: p.CreatedAt,
	}
}

// toRefreshTokenPO 是写方向的转换；CreatedAt 携带域值（应用层已设），
// autoCreateTime 对非零值不覆盖，零值时兜底生成。
func toRefreshTokenPO(t user.RefreshToken) po.RefreshToken {
	return po.RefreshToken{
		TokenHash: t.TokenHash,
		UserID:    t.UserID,
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
	}
}
