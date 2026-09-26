// Package po 集中定义 MySQL 各表的持久化模型，仅由 persistence/mysql 的仓储使用，
// 是存储形态与领域模型之间的转换边界（术语见 backend/CONTEXT.md；约束见 backend/AGENTS.md 持久化章节）。
// 依赖方向：只依赖 domain，是基础设施层内的叶子包。
// 全部表结构与字段显式声明 gorm 标签，schema 唯一来源是 po + AutoMigrate。
package po

import "time"

// User 是 users 表的行形状。
type User struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Email        string    `gorm:"column:email;type:varchar(255);not null;uniqueIndex:uq_users_email"`
	PasswordHash string    `gorm:"column:password_hash;type:varchar(100);not null"`
	Nickname     string    `gorm:"column:nickname;type:varchar(64);not null"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamp;not null;autoCreateTime"`
}

// TableName 显式声明表名。
func (User) TableName() string { return "users" }
