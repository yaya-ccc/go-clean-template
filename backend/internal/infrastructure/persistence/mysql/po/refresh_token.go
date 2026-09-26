package po

import "time"

// RefreshToken 是 refresh_tokens 表的行形状。
type RefreshToken struct {
	TokenHash string    `gorm:"column:token_hash;primaryKey;type:char(64);not null"`
	UserID    int64     `gorm:"column:user_id;type:bigint unsigned;not null;index:idx_refresh_tokens_user_id"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamp;not null"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;not null;autoCreateTime"`
}

// TableName 显式声明表名。
func (RefreshToken) TableName() string { return "refresh_tokens" }
