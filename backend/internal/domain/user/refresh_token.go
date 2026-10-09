package user

import (
	"context"
	"time"
)

// RefreshToken 是可吊销的长期登录凭据：客户端持有原始随机串，服务端只存其哈希。
// 库中不存在该哈希（未登录、已登出或已过期清理）即视为未登录。
type RefreshToken struct {
	TokenHash string // 原始 token 的 SHA-256 十六进制
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

// RefreshTokenRepository 是刷新凭据的仓储契约，由基础设施层实现。
// 错误契约：Find 未命中不是错误，返回 (nil, nil)；
// Delete 区分「无记录删除」（幂等成功）与「存储故障」（返回错误）；
// 未知失败统一 internal_error 并保留原始 cause。
type RefreshTokenRepository interface {
	// Save 写入一条刷新凭据。
	Save(ctx context.Context, t RefreshToken) error
	// Find 按哈希查找刷新凭据；未命中返回 (nil, nil)。
	Find(ctx context.Context, tokenHash string) (*RefreshToken, error)
	// Delete 按哈希删除刷新凭据（登出吊销）；无记录时静默成功（幂等），故障时报错。
	Delete(ctx context.Context, tokenHash string) error
}
