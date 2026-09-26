package mysql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go-clean-template/backend/internal/infrastructure/persistence/mysql/po"
	"go-clean-template/backend/internal/kernel/apperr"

	drivermysql "github.com/go-sql-driver/mysql"
)

// dupErr 构造带真实结构的驱动 1062 错误；测试不使用缺 SQLState 的简化样本。
func dupErr(entry, key string) *drivermysql.MySQLError {
	return &drivermysql.MySQLError{
		Number:   1062,
		SQLState: [5]byte{'2', '3', '0', '0', '0'},
		Message:  fmt.Sprintf("Duplicate entry '%s' for key '%s'", entry, key),
	}
}

func TestTranslateUserCreateDuplicate(t *testing.T) {
	// 命名索引与带表前缀两种物理形态都应直接产出业务码 email_taken。
	for _, key := range []string{"uq_users_email", "users.uq_users_email"} {
		got := translateUserCreateError(dupErr("alice@example.com", key))
		if !apperr.CodeIs(got, apperr.ErrEmailTaken) {
			t.Fatalf("索引 %s 应产出 email_taken，得到 %s", key, apperr.CodeOf(got))
		}
		var native *drivermysql.MySQLError
		if !errors.As(got, &native) || native.Number != 1062 {
			t.Fatal("应保留原始驱动错误链")
		}
	}
	// 白名单外的重复键（PRIMARY、其它表同名索引、未知索引、重复值含索引名）
	// 一律 database_error，不猜字段。
	for name, err := range map[string]error{
		"主键冲突":          dupErr("uq_users_email", "PRIMARY"),
		"重复值包含索引名":      dupErr("x for key 'uq_users_email'", "PRIMARY"),
		"其它表同名索引":       dupErr("x", "other_table.uq_users_email"),
		"未知索引":          dupErr("x", "idx_new"),
		"缺 SQLState 伪造": &drivermysql.MySQLError{Number: 1062, Message: "Duplicate entry 'a' for key 'uq_users_email'"},
	} {
		if got := translateUserCreateError(err); !apperr.CodeIs(got, apperr.ErrDatabase) {
			t.Fatalf("%s 应为 database_error，得到 %s", name, apperr.CodeOf(got))
		}
	}
}

func TestTranslateUserCreateNonDuplicate(t *testing.T) {
	// 非 1062 驱动错误同样归入 database_error。
	other := &drivermysql.MySQLError{Number: 1045, SQLState: [5]byte{'2', '8', '0', '0', '0'}, Message: "denied"}
	if got := translateUserCreateError(other); !apperr.CodeIs(got, apperr.ErrDatabase) {
		t.Fatalf("非 1062 应为 database_error，得到 %s", apperr.CodeOf(got))
	}
}

func TestNormalizeStoreErrorContext(t *testing.T) {
	wrapped := fmt.Errorf("query: %w", context.Canceled)
	if got := normalizeStoreError(wrapped, "op"); !apperr.CodeIs(got, apperr.ErrRequestCanceled) {
		t.Fatalf("取消应保留分类，得到 %s", apperr.CodeOf(got))
	}
	deadline := fmt.Errorf("query: %w", context.DeadlineExceeded)
	if got := normalizeStoreError(deadline, "op"); !apperr.CodeIs(got, apperr.ErrOperationTimeout) {
		t.Fatalf("超时应保留分类，得到 %s", apperr.CodeOf(got))
	}
	if got := normalizeStoreError(errors.New("conn refused"), "op"); !apperr.CodeIs(got, apperr.ErrDatabase) {
		t.Fatalf("未知失败应为 database_error，得到 %s", apperr.CodeOf(got))
	}
	if normalizeStoreError(nil, "op") != nil {
		t.Fatal("nil 输入应返回 nil")
	}
}

// TestIndexContract 钉住仓储边界分类的索引常量与 po 的 uniqueIndex tag 字面量一致，
// 防止 po 侧改名后分类静默漂移到 database_error。
func TestIndexContract(t *testing.T) {
	f, ok := reflect.TypeOf(po.User{}).FieldByName("Email")
	if !ok {
		t.Fatal("po.User 应有 Email 字段")
	}
	if tag := f.Tag.Get("gorm"); !strings.Contains(tag, "uniqueIndex:"+usersEmailIndex) {
		t.Fatalf("po.User Email tag 应含 uniqueIndex:%s，得到 %q", usersEmailIndex, tag)
	}
}
