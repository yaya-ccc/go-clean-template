package response

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"

	"go-clean-template/backend/internal/kernel/apperr"
)

// TestErrorRegistryCoverage 防止新增错误码漏登记，或删除错误码后留下悬空翻译。
func TestErrorRegistryCoverage(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", "kernel", "apperr", "codes.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[apperr.ErrCode]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		typ, ok := spec.Type.(*ast.Ident)
		if !ok || typ.Name != "ErrCode" || len(spec.Names) != 1 || len(spec.Values) != 1 {
			t.Fatal("注册表中的码必须显式声明为单个 ErrCode 常量")
		}
		for _, value := range spec.Values {
			lit, ok := value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Fatal("错误码必须是显式字符串常量")
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			code := apperr.ErrCode(s)
			if codes[code] {
				t.Fatalf("错误码重复：%s", code)
			}
			codes[code] = true
			if entry, ok := errTable[code]; !ok || entry.status < 400 || entry.message == "" {
				t.Fatalf("错误码缺少完整翻译：%s", code)
			}
		}
		return false
	})
	if len(codes) == 0 || len(codes) != len(errTable) {
		t.Fatalf("注册表与翻译表数量不符：%d %d", len(codes), len(errTable))
	}
	for code := range errTable {
		if !codes[code] {
			t.Fatalf("翻译表悬空键：%s", code)
		}
	}
}
