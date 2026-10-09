// Package mysql 提供与 MySQL 的连接建立与仓储实现，属于基础设施层。
package mysql

import (
	"database/sql"

	"go-clean-template/backend/internal/infrastructure/persistence/mysql/po"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 将 DSN 解析和连接初始化失败在技术边界转换为项目错误。
// SkipDefaultTransaction 关闭单条语句的隐式事务包装；跨步骤原子性由显式事务承担。
// TranslateError 关闭：驱动 1062 会被 GORM 直接替换为 ErrDuplicatedKey 并丢失索引信息与原始
// cause，因此由本包适配层直接识别原生错误（errors.go)）。
func Open(dsn string) (*gorm.DB, error) {
	g, err := gorm.Open(mysql.New(mysql.Config{
		DriverName: "mysql",
		DSN:        dsn,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         false,
		Logger:                 logger.Discard, // 过程日志由表现层中间件统一负责，GORM 不重复打日志
	})
	if err != nil {
		return nil, normalizeStoreError(err, "mysql.open")
	}
	return g, nil
}

// Migrate 按 po 包声明的表结构执行 AutoMigrate：幂等，只增列/索引，不改不删已有结构。
// 迁移清单在此集中维护，main 只调用不感知 po。
func Migrate(g *gorm.DB) error {
	return normalizeStoreError(g.AutoMigrate(&po.User{}, &po.RefreshToken{}), "mysql.migrate")
}

// SQLDB 取回底层 *sql.DB，供健康检查等仅需要标准库句柄的场合。
func SQLDB(g *gorm.DB) (*sql.DB, error) {
	db, err := g.DB()
	if err != nil {
		return nil, normalizeStoreError(err, "mysql.sql_db")
	}
	return db, nil
}
