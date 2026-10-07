// Package fakes 存放 user 仓储契约（domain/user.Repository 等）的内存假实现，
// 供应用层与表现层测试共享。
//
// 错误形态与 MySQL 仓储对齐：Create 冲突返回业务码 email_taken；Find 未命中
// 返回 (nil, nil)；伪造数据库故障返回 internal_error。
package fakes

import (
	"context"
	"sync"
	"time"

	"go-clean-template/backend/internal/domain/user"
	"go-clean-template/backend/internal/kernel/apperr"
)

// Users 是 user.Repository 的内存假件；并发安全。
type Users struct {
	mu      sync.Mutex
	byEmail map[string]*user.User
	nextID  int64
}

// NewUsers 构造空的用户仓储假件。
func NewUsers() *Users { return &Users{byEmail: map[string]*user.User{}} }

// Create 插入用户；同邮箱已存在时与 MySQL 仓储同样产出 email_taken。
func (m *Users) Create(_ context.Context, email, hash, nickname string) (*user.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byEmail[email]; ok {
		return nil, apperr.ErrEmailTaken.With("fake.user.create")
	}
	m.nextID++
	u := &user.User{ID: m.nextID, Email: email, PasswordHash: hash, Nickname: nickname, CreatedAt: time.Now()}
	m.byEmail[email] = u
	return u, nil
}

// FindByEmail 按邮箱查找；未命中返回 (nil, nil)。
func (m *Users) FindByEmail(_ context.Context, email string) (*user.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.byEmail[email]; ok {
		return u, nil
	}
	return nil, nil
}

// FindByID 按主键查找；未命中返回 (nil, nil)。
func (m *Users) FindByID(_ context.Context, id int64) (*user.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

// UnknownDupUsers 在 Users 之上把 Create 替换为 internal_error：
// 白名单外的写入冲突经仓储边界分类统一归入数据库故障，应用层透传。
type UnknownDupUsers struct{ *Users }

// Create 恒返回 internal_error，模拟未被索引白名单识别的重复键。
func (m *UnknownDupUsers) Create(_ context.Context, email, hash, nickname string) (*user.User, error) {
	return nil, apperr.ErrInternal.With("fake.user.create")
}

// RefreshTokens 是 user.RefreshTokenRepository 的内存假件；并发安全。
type RefreshTokens struct {
	mu     sync.Mutex
	byHash map[string]user.RefreshToken
}

// NewRefreshTokens 构造空的刷新凭据仓储假件。
func NewRefreshTokens() *RefreshTokens { return &RefreshTokens{byHash: map[string]user.RefreshToken{}} }

// Save 写入刷新凭据，按哈希覆盖。
func (m *RefreshTokens) Save(_ context.Context, t user.RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byHash[t.TokenHash] = t
	return nil
}

// Find 按哈希查找；未命中返回 (nil, nil)。
func (m *RefreshTokens) Find(_ context.Context, h string) (*user.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.byHash[h]; ok {
		return &t, nil
	}
	return nil, nil
}

// Delete 按哈希删除；不存在时幂等成功。
func (m *RefreshTokens) Delete(_ context.Context, h string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byHash, h)
	return nil
}

// FailingRefreshTokens 的 Save 恒失败，用于注入签发链路故障；
// 其余行为与 RefreshTokens 一致。
type FailingRefreshTokens struct {
	*RefreshTokens
	err error
}

// NewFailingRefreshTokens 构造 Save 恒返回 err 的假件。
func NewFailingRefreshTokens(err error) *FailingRefreshTokens {
	return &FailingRefreshTokens{RefreshTokens: NewRefreshTokens(), err: err}
}

// Save 保持真实仓储的错误契约，注入的裸错误在此转换，已分类错误不重复包装。
func (m *FailingRefreshTokens) Save(_ context.Context, t user.RefreshToken) error {
	if m.err == nil {
		return nil
	}
	if _, ok := apperr.First(m.err); ok {
		return m.err
	}
	return apperr.ErrInternal.With("fake.refresh.save", apperr.WithCause(m.err))
}
