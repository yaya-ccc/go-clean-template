package apperr

import (
	"errors"
	"strings"
	"testing"
)

// TestCodeSentinelsDistinct 校验基础码互不相同且与业务码可区分。
func TestCodeSentinelsDistinct(t *testing.T) {
	all := []ErrCode{ErrInvalidInput, ErrUnauthorized, ErrInternal, ErrRouteNotFound,
		ErrMethodNotAllowed, ErrBadCredentials, ErrEmailTaken}
	for i, a := range all {
		for j, b := range all {
			if i != j && a == b {
				t.Fatalf("%s 与 %s 不应同码", a, b)
			}
		}
	}
}

// TestWithDerivesTypedError 校验 With 从码派生实例：携带 op、可被 First/CodeOf 读取，
// 且哨兵（裸 ErrCode）经 First 不可见——裸返回哨兵时调用方拿到的是码本身。
func TestWithDerivesTypedError(t *testing.T) {
	e := ErrBadCredentials.With("app.login")
	if e.Message != "app.login" || e.Code != ErrBadCredentials {
		t.Fatalf("With 应携带 code 与 op，得到 %+v", e)
	}
	if _, ok := First(ErrBadCredentials); ok {
		t.Fatal("裸哨兵不应被 First 识别为 *Error")
	}
	if got := CodeOf(ErrBadCredentials); got != "" {
		t.Fatalf("裸哨兵的 CodeOf 应为空码，得到 %s", got)
	}
	if got := CodeOf(e); got != ErrBadCredentials {
		t.Fatalf("派生实例的 CodeOf 应为 bad_credentials，得到 %s", got)
	}
}

// TestWithCauseKeepsChain 校验 WithCause 保留因果实链（errors.Is/As 穿透）；
// nil 输入无效果，不产生假失败。
func TestWithCauseKeepsChain(t *testing.T) {
	cause := errors.New("dial tcp")
	e := ErrInternal.With("mysql query", WithCause(cause))
	if !errors.Is(e, cause) {
		t.Fatal("WithCause 应保留原因链")
	}
	if !errors.Is(e, ErrInternal) {
		t.Fatal("同码即 Is：派生实例应匹配哨兵")
	}
	noCause := ErrInternal.With("op", WithCause(nil))
	if noCause.Unwrap() != nil {
		t.Fatal("WithCause(nil) 应无效果")
	}
}

// TestIsMatchesSameCodeCrossPointer 校验同 code 不同指针也命中，且跨形态：
// *Error 匹配裸 ErrCode 哨兵、反向亦然。
func TestIsMatchesSameCodeCrossPointer(t *testing.T) {
	fresh := ErrUnauthorized.With("created elsewhere")
	if !errors.Is(fresh, ErrUnauthorized) {
		t.Fatal("同码的独立实例应命中 sentinel")
	}
	if errors.Is(ErrInvalidInput.With("x"), ErrUnauthorized) {
		t.Fatal("异码不应命中")
	}
	if !errors.Is(errors.Join(ErrBadCredentials.With("a")), ErrBadCredentials) {
		t.Fatal("链上派生实例应命中裸哨兵")
	}
}

// TestIsTypedNilTargetNotPanic 校验 typed-nil 目标不导致 panic。
func TestIsTypedNilTargetNotPanic(t *testing.T) {
	var nilErr *Error
	err := ErrUnauthorized.With("x")
	if err.Is(nilErr) {
		t.Fatal("typed-nil 目标不应匹配")
	}
}

// TestErrorOmitsCauseText 校验 Error() 不拼接原始 cause 文本，
// 保留 MySQL 原始错误后其敏感内容不能经普通字符串输出泄漏；
// op 为空时只输出 code，不打尾随冒号。
func TestErrorOmitsCauseText(t *testing.T) {
	e := ErrInternal.With("mysql query", WithCause(errors.New("secret value 'a@b.com'")))
	if s := e.Error(); strings.Contains(s, "a@b.com") {
		t.Fatalf("Error() 不应泄漏 cause 文本：%s", s)
	}
	if s := e.Error(); s != "internal_error: mysql query" {
		t.Fatalf("带 op 输出应为 code: op，得到 %s", s)
	}
	if s := ErrUnauthorized.Error(); s != "unauthorized" {
		t.Fatalf("裸哨兵应只输出 code，得到 %s", s)
	}
}

// TestFirstAndCodeOf 校验 First/CodeOf 只取最外层项目错误：
// email_taken 包 duplicate 时当前语义是 email_taken；
// 外层 internal_error 包内层公开码时仍是 internal_error，不向内回退。
func TestFirstAndCodeOf(t *testing.T) {
	inner := ErrInternal.With("mysql.user.create")
	mid := ErrCode("email_taken").With("auth.register.email_conflict", WithCause(inner))
	if CodeOf(mid) != "email_taken" {
		t.Fatalf("当前语义应为 email_taken，得到 %s", CodeOf(mid))
	}
	outer := ErrInternal.With("auth.register.unclassified", WithCause(mid))
	if CodeOf(outer) != "internal_error" {
		t.Fatalf("外层语义应取胜，得到 %s", CodeOf(outer))
	}
}

// TestDetailOfOnlyCurrentLayer 校验 DetailOf 只读当前语义层 detail，不向内搜索。
func TestDetailOfOnlyCurrentLayer(t *testing.T) {
	type key struct{ K string }
	inner := ErrInternal.With("mysql.user.create", WithDetail(key{K: "email"}))
	outer := ErrCode("email_taken").With("auth.register", WithCause(inner))
	if _, ok := DetailOf[key](outer); ok {
		t.Fatal("外层未携带该 detail 时不应向内读到仓储层 detail")
	}
	if got, ok := DetailOf[key](inner); !ok || got.K != "email" {
		t.Fatalf("当前层 detail 应可读，得到 %+v %v", got, ok)
	}
}
