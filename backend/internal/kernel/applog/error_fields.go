package applog

import (
	"fmt"

	"go-clean-template/backend/internal/kernel/apperr"

	"go.uber.org/zap"
)

// maxErrorChainDepth 是编码器遍历错误链的深度上限，防御异常深的包装链。
const maxErrorChainDepth = 10

// ErrorFields 把错误链编码为 zap 字段：项目错误输出 code + 静态上下文，
// 链尾的第三方/标准库原因记录类型与原文——原文含驱动错误号与数据值，
// 排障时直接可读（接受数据值进日志换取诊断直观）；
// 不反射整个对象、不序列化 detail。
// 供过程日志中间件在请求摘要中输出统一诊断。
func ErrorFields(err error) []zap.Field {
	if err == nil {
		return nil
	}
	type chainItem struct {
		Code string `json:"code"`
		Op   string `json:"op"`
	}
	var (
		chain     []chainItem
		causeType string
		causeMsg  string
	)

	for cur, depth := err, 0; cur != nil && depth < maxErrorChainDepth; depth++ {
		if e, ok := apperr.First(cur); ok {
			chain = append(chain, chainItem{Code: string(e.Code), Op: e.Message})
			cur = e.Unwrap()
			continue
		}
		// 链尾的第三方/标准库原因记类型与原文，排障时直接可见驱动错误号与数据值。
		causeType = fmt.Sprintf("%T", cur)
		causeMsg = cur.Error()
		break
	}

	fields := []zap.Field{zap.Any("error_chain", chain)}
	if causeType != "" {
		fields = append(fields, zap.String("cause_type", causeType), zap.String("cause", causeMsg))
	}
	return fields
}
