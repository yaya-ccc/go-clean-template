package mysql

import (
	"context"
	"errors"

	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/infrastructure/persistence/mysql/po"
	"go-clean-template/backend/internal/kernel/apperr"

	"gorm.io/gorm"
)

// RefreshTokenRepo 是刷新凭据仓储的 GORM 实现；实例可绑定到根连接或某个显式事务。
type RefreshTokenRepo struct{ db *gorm.DB }

// NewRefreshTokenRepo 构造刷新凭据仓储；db 为必填，nil 视为组装错误。
func NewRefreshTokenRepo(db *gorm.DB) (*RefreshTokenRepo, error) {
	if db == nil {
		return nil, apperr.ErrInternal.With("mysql.NewRefreshTokenRepo: db must not be nil")
	}
	return &RefreshTokenRepo{db: db}, nil
}

// Save 写入一条刷新凭据；token_hash 为主键，冲突（同 token 重复保存）按内部失败处理，
// 不映射为任何用户字段占用（随机哈希冲突属生成侧异常)）。
func (r *RefreshTokenRepo) Save(ctx context.Context, t user.RefreshToken) error {
	p := toRefreshTokenPO(t)
	if err := r.db.WithContext(ctx).Create(&p).Error; err != nil {
		return normalizeStoreError(err, "mysql.refresh_token.save")
	}
	return nil
}

// Find 按哈希查找刷新凭据；未命中不是错误，返回 (nil, nil)。
func (r *RefreshTokenRepo) Find(ctx context.Context, tokenHash string) (*user.RefreshToken, error) {
	var p po.RefreshToken
	err := r.db.WithContext(ctx).First(&p, "token_hash = ?", tokenHash).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, normalizeStoreError(err, "mysql.refresh_token.find")
	}
	t := toDomainRefreshToken(p)
	return &t, nil
}

// Delete 按哈希删除刷新凭据（登出吊销）；无记录时幂等成功，故障时报错不吞错。
func (r *RefreshTokenRepo) Delete(ctx context.Context, tokenHash string) error {
	err := r.db.WithContext(ctx).Delete(&po.RefreshToken{}, "token_hash = ?", tokenHash).Error
	if err != nil {
		return normalizeStoreError(err, "mysql.refresh_token.delete")
	}
	return nil
}
