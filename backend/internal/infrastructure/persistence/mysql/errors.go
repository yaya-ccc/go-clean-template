package mysql

import (
	"context"
	"errors"
	"regexp"

	"go-clean-template/backend/internal/kernel/apperr"

	drivermysql "github.com/go-sql-driver/mysql"
)

// usersEmailIndex 是 users 表邮箱唯一索引的物理名，须与 po.User 的
// uniqueIndex tag 字面量一致；由 TestIndexContract 钉住，防止 po 侧
// 静默漂移。
const usersEmailIndex = "uq_users_email"

// duplicateKeySuffix 只匹配消息尾部的 "for key '...'" 部分，不提取 Duplicate entry
// 中的重复值：重复值里出现的任意文本不能被当作索引名。
var duplicateKeySuffix = regexp.MustCompile(`(?s) for key '([A-Za-z0-9_.]+)'\s*$`)

// normalizeStoreError 归一化仓储未知失败与标准库原因：
// context 取消/超时保留其分类，其余（含全部数据库起源故障）为 database_error；
// 原始错误经 WithCause 保留在链上，由日志输出原文排障。
func normalizeStoreError(err error, op string) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return apperr.ErrRequestCanceled.With(op, apperr.WithCause(err))
	case errors.Is(err, context.DeadlineExceeded):
		return apperr.ErrOperationTimeout.With(op, apperr.WithCause(err))
	default:
		return apperr.ErrDatabase.With(op, apperr.WithCause(err))
	}
}

// isDuplicateKeyError 判定驱动 1062 重复键错误（含 SQLState 23000 校验）。
func isDuplicateKeyError(err error) (*drivermysql.MySQLError, bool) {
	var native *drivermysql.MySQLError
	if !errors.As(err, &native) || native == nil {
		return nil, false
	}
	if native.Number != 1062 || string(native.SQLState[:]) != "23000" {
		return nil, false
	}
	return native, true
}

// translateUserCreateError 翻译 users 表写入失败：白名单内的邮箱唯一索引冲突
// 直接产出业务码 email_taken；白名单外的重复键（PRIMARY、改名索引等）与
// 其余数据库故障统一 database_error。
func translateUserCreateError(err error) error {
	if err == nil {
		return nil
	}
	if native, ok := isDuplicateKeyError(err); ok {
		if match := duplicateKeySuffix.FindStringSubmatch(native.Message); len(match) == 2 &&
			(match[1] == usersEmailIndex || match[1] == "users."+usersEmailIndex) {
			return apperr.ErrEmailTaken.With("mysql.user.create", apperr.WithCause(err))
		}
		return apperr.ErrDatabase.With("mysql.user.create.unrecognized_index", apperr.WithCause(err))
	}
	return normalizeStoreError(err, "mysql.user.create")
}
