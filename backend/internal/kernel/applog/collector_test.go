package applog

import (
	"context"
	"testing"
	"time"

	"go-clean-template/backend/internal/kernel/apperr"
)

func TestAddAcceptsLimitedTypes(t *testing.T) {
	ts := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	ctx := NewCollector(context.Background())
	Add(ctx, "s", String("x"))
	Add(ctx, "i", Int64(3))
	Add(ctx, "f", Float64(1.5))
	Add(ctx, "b", Bool(true))
	Add(ctx, "t", Time(ts))
	Add(ctx, "e", Err(apperr.ErrCode("internal_error").With("boom")))

	got := Fields(ctx)
	// 错误项输出 code/op 两字段对象，不再拼接为单字符串。
	errVal, ok := got["e"].(map[string]any)
	if !ok || errVal["code"] != "internal_error" || errVal["op"] != "boom" {
		t.Errorf("fields()[\"e\"] 应为 {code,op}，得到 %v", got["e"])
	}
	if v, ok := got["t"]; !ok || v != "2026-09-23T10:00:00Z" {
		t.Errorf("fields()[\"t\"] = %v, want 时间字符串", v)
	}
}

func TestAddWithoutCollectorIsNoop(t *testing.T) {
	Add(context.Background(), "k", String("v")) // 不应 panic
	if f := Fields(context.Background()); len(f) != 0 {
		t.Errorf("无收集器时 fields() 应为空，得到 %v", f)
	}
}

func TestAddRejectedReservedKeys(t *testing.T) {
	ctx := NewCollector(context.Background())
	Add(ctx, "request_id", String("biz-1"))
	Add(ctx, "latency_ms", Int64(5))
	Add(ctx, "user_id", Int64(9))
	Add(ctx, "ordinary_key", String("ok"))

	got := Fields(ctx)
	if _, ok := got["request_id"]; ok {
		t.Error("保留键 request_id 应被拒绝")
	}
	if _, ok := got["latency_ms"]; ok {
		t.Error("保留键 latency_ms 应被拒绝")
	}
	if _, ok := got["user_id"]; ok {
		t.Error("保留键 user_id 应被拒绝")
	}
	if v, ok := got["ordinary_key"]; !ok || v != "ok" {
		t.Errorf("普通键应被接受，得到 %v", v)
	}
}

func TestAddNonSnakeCaseKeyRejected(t *testing.T) {
	ctx := NewCollector(context.Background())
	for _, k := range []string{"WrongCount", "wrong count", "wrong-count", "wrongCount", "中文键"} {
		Add(ctx, k, String("v"))
	}
	if got := Fields(ctx); len(got) != 0 {
		t.Errorf("非 snake_case 键应全部被拒绝，得到 %v", got)
	}
	// 合法边界：单段小写词、数字、下划线
	Add(ctx, "ai_usage_v2", String("ok"))
	if v, ok := Fields(ctx)["ai_usage_v2"]; !ok || v != "ok" {
		t.Errorf("合法 snake_case 键应被接受，得到 %v", v)
	}
}

func TestAddSameKeyOverwrites(t *testing.T) {
	ctx := NewCollector(context.Background())
	Add(ctx, "count", Int64(1))
	Add(ctx, "count", Int64(2))
	if v := Fields(ctx)["count"]; v != int64(2) {
		t.Errorf("同键重复 Add 应覆盖，得到 %v", v)
	}
}

func TestFieldsWithoutCollectorReturnsEmpty(t *testing.T) {
	if f := Fields(context.Background()); len(f) != 0 {
		t.Errorf("无收集器时 fields() 应为空，得到 %v", f)
	}
}
