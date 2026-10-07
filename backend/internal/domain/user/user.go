// Package user 定义用户与登录凭据的领域模型及仓储契约。
package user

import (
	"context"
	"time"
)

// User 是平台的登录主体：邮箱全局唯一，密码只允许以哈希形态存在。
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Nickname     string
	CreatedAt    time.Time
}

// Repository 是用户实体的仓储契约，由基础设施层实现。
// 错误契约：
//   - Find 类查询未命中不是错误：返回 (nil, nil)，由应用层按用例决定最终语义。
//   - 写入唯一冲突：由仓储边界分类直接产出业务码
//     （邮箱唯一索引冲突 → email_taken），调用方透传即可。
//   - 读写未知失败与未识别的写入拒绝：返回 internal_error，保留原始 cause。
type Repository interface {
	// Create 持久化新用户；唯一冲突按上述契约返回；created_at 在应用侧生成，
	// 插入后无需回读。
	Create(ctx context.Context, email, passwordHash, nickname string) (*User, error)
	// FindByEmail 按邮箱查找用户；未命中返回 (nil, nil)。
	FindByEmail(ctx context.Context, email string) (*User, error)
	// FindByID 按主键查找用户；未命中返回 (nil, nil)。
	FindByID(ctx context.Context, id int64) (*User, error)
}
