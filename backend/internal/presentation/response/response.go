// Package response 是表现层共享的错误翻译与响应渲染基础设施：
// 错误码翻译表、统一 JSON 渲染与请求体绑定，不含业务语义。
package response

import (
	"net/http"

	"go-clean-template/backend/internal/kernel/apperr"

	"github.com/gin-gonic/gin"
)

// OK 输出成功 JSON 响应，data 直接作为响应体（不包额外信封）。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Ack 输出无数据负载端点的统一执行成功响应 200 {"status":"ok"}：
// logout、healthz 这类只需表明动作已完成的端点共用，不再各自散落同构 DTO。
func Ack(c *gin.Context) {
	OK(c, ackBody)
}

// ackBody 是 Ack 的固定响应视图；包级私有，杜绝调用点拼装变体。
var ackBody = struct {
	Status string `json:"status"`
}{Status: "ok"}

// ErrorResponse 是错误响应的显式 DTO：code/message 为既有契约，
// request_id 为可选增量字段（violations 通道已删）。
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// errEntry 是翻译表中一个错误码的对外契约：HTTP 状态码 + 用户文案。
type errEntry struct {
	status  int
	message string
}

// errTable 是错误码 → 对外响应的唯一来源：一码一状态一文案。
// 新增错误码时在此登记；语义不同就造新的 code，禁止调用点覆盖文案。
var errTable = map[apperr.ErrCode]errEntry{
	apperr.ErrInvalidInput:       {http.StatusBadRequest, "请求参数无效"},
	apperr.ErrBadCredentials:     {http.StatusUnauthorized, "邮箱或密码错误"},
	apperr.ErrUnauthorized:       {http.StatusUnauthorized, "登录已过期，请重新登录"},
	apperr.ErrEmailTaken:         {http.StatusConflict, "邮箱已被注册"},
	apperr.ErrInternal:           {http.StatusInternalServerError, "服务器内部错误"},
	apperr.ErrDatabase:           {http.StatusInternalServerError, "服务器内部错误"},
	apperr.ErrServiceUnavailable: {http.StatusServiceUnavailable, "服务暂不可用"},
	apperr.ErrOperationTimeout:   {http.StatusServiceUnavailable, "请求处理超时，请稍后重试"},
	apperr.ErrRouteNotFound:      {http.StatusNotFound, "接口不存在"},
	apperr.ErrMethodNotAllowed:   {http.StatusMethodNotAllowed, "请求方法不允许"},
}

// Fail 将任意错误翻译为统一 JSON 错误响应。
// 固定顺序：只取最外层项目错误 → 查公开翻译表 → 未登记码统一 internal_error；
// 响应只写一次（已提交则不再追加）。
// 外层 internal_error 包裹的内层公开码必须返回 500：外层转换代表当前用例决定。
func Fail(c *gin.Context, err error) {
	if err == nil || c.Writer.Written() {
		return
	}
	_ = c.Error(err) // 供请求摘要读取；日志输出走安全编码器
	e, ok := apperr.First(err)
	if !ok {
		WriteInternal(c)
		return
	}
	entry, ok := errTable[e.Code]
	if !ok {
		WriteInternal(c)
		return
	}
	c.JSON(entry.status, ErrorResponse{Code: string(e.Code), Message: entry.message})
}

// WriteInternal 输出统一 internal_error 契约，供 Fail 兜底与 panic 恢复共用。
func WriteInternal(c *gin.Context) {
	entry := errTable[apperr.ErrInternal]
	c.AbortWithStatusJSON(entry.status, ErrorResponse{
		Code: string(apperr.ErrInternal), Message: entry.message,
	})
}
