package mysql

import (
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
)

func TestOpenConvertsDriverInitializationFailure(t *testing.T) {
	// 非法 DSN 在建连前失败，确定性验证第三方错误转换，无需真实 MySQL。
	db, err := Open("invalid-dsn")
	if db != nil || !apperr.CodeIs(err, apperr.ErrInternal) {
		t.Fatalf("初始化失败应返回 nil 与 internal_error，得到 %v %v", db, err)
	}
	project, _ := apperr.First(err)
	if project.Unwrap() == nil {
		t.Fatal("初始化失败必须保留驱动原始原因供启动日志排障")
	}
}
