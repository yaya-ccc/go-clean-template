package mysql

import (
	"context"
	"errors"
	"time"

	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/infrastructure/persistence/mysql/po"
	"go-clean-template/backend/internal/kernel/apperr"

	"gorm.io/gorm"
)

// UserRepo 是用户仓储的 GORM 实现；实例可绑定到根连接或某个显式事务。
type UserRepo struct{ db *gorm.DB }

// NewUserRepo 构造用户仓储；db 为必填，nil 视为组装错误。
func NewUserRepo(db *gorm.DB) (*UserRepo, error) {
	if db == nil {
		return nil, apperr.ErrInternal.With("mysql.NewUserRepo: db must not be nil")
	}
	return &UserRepo{db: db}, nil
}

// Create 插入新用户；created_at 由 autoCreateTime 在应用侧生成并随 INSERT 携带
// （GORM 自动回填内存值，无需回读)）。
// 唯一冲突由 translateUserCreateError 直接分类业务码（邮箱索引 → email_taken，
// 未识别重复键与数据库故障 → database_error)）。
func (r *UserRepo) Create(ctx context.Context, email, passwordHash, nickname string) (*user.User, error) {
	// created_at 列为秒级 timestamp，MySQL 对小数秒四舍五入会进位到下一秒；
	// 预置秒级截断值（autoCreateTime 对非零值不覆盖），保证内存值、库内值
	// 与 RFC3339 秒级输出三方一致，无需回读。
	p := po.User{Email: email, PasswordHash: passwordHash, Nickname: nickname,
		CreatedAt: time.Now().Truncate(time.Second)}
	if err := r.db.WithContext(ctx).Create(&p).Error; err != nil {
		return nil, translateUserCreateError(err)
	}
	u := toDomainUser(p)
	return &u, nil
}

// FindByEmail 按邮箱查找用户；未命中不是错误，返回 (nil, nil)。
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return r.findOne(ctx, r.db, "email = ?", email)
}

// FindByID 按主键查找用户；未命中不是错误，返回 (nil, nil)。
func (r *UserRepo) FindByID(ctx context.Context, id int64) (*user.User, error) {
	return r.findOne(ctx, r.db, "id = ?", id)
}

// findOne 执行单行查询并扫描为用户；无行返回 (nil, nil)，未命中事实由应用层
// 判空产出，未知失败经归一化。
func (r *UserRepo) findOne(ctx context.Context, db *gorm.DB, query string, arg any) (*user.User, error) {
	var p po.User
	err := db.WithContext(ctx).Where(query, arg).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, normalizeStoreError(err, "mysql.user.find")
	}
	u := toDomainUser(p)
	return &u, nil
}
