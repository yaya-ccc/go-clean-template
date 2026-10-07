// Package response 统一错误翻译与 JSON 渲染，不决定业务错误语义。
package response

import (
	"net/http"

	"go-clean-template/backend/internal/kernel/apperr"

	"github.com/gin-gonic/gin"
)

// OK 直接输出成功视图，不包额外信封；取消或已提交的响应不再写入。
func OK(c *gin.Context, data any) {
	if c.Writer.Written() || c.Request.Context().Err() != nil {
		return
	}
	c.JSON(http.StatusOK, data)
}

// Ack 为无数据负载动作提供唯一成功视图，不要求用例制造空 DTO。
func Ack(c *gin.Context) {
	OK(c, ackBody)
}

var ackBody = struct {
	Status string `json:"status"`
}{Status: "ok"}

// ErrorResponse 的请求标识由渲染调用方注入，必须与响应头和日志同源。
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type errEntry struct {
	status  int
	message string
}

// 一码一状态一文案；系统起源与原始文本只存在于日志诊断通道。
var errTable = map[apperr.ErrCode]errEntry{
	apperr.ErrInvalidInput:     {http.StatusBadRequest, "请求参数无效"},
	apperr.ErrBadCredentials:   {http.StatusUnauthorized, "邮箱或密码错误"},
	apperr.ErrUnauthorized:     {http.StatusUnauthorized, "登录已过期，请重新登录"},
	apperr.ErrEmailTaken:       {http.StatusConflict, "邮箱已被注册"},
	apperr.ErrInternal:         {http.StatusInternalServerError, "服务器内部错误"},
	apperr.ErrRouteNotFound:    {http.StatusNotFound, "接口不存在"},
	apperr.ErrMethodNotAllowed: {http.StatusMethodNotAllowed, "请求方法不允许"},
}

// Fail 按最外层语义渲染；裸错误和未登记码安全降级，日志保留原始 cause。
// 请求已取消时只登记诊断，不再尝试写 JSON；已提交响应不追加第二份正文。
func Fail(c *gin.Context, err error, requestID string) {
	if err == nil || c.Writer.Written() {
		return
	}
	e, ok := apperr.First(err)
	if !ok {
		err = apperr.ErrInternal.With("response.unclassified", apperr.WithCause(err))
		e, _ = apperr.First(err)
	}
	entry, ok := errTable[e.Code]
	if !ok {
		err = apperr.ErrInternal.With("response.unregistered_code", apperr.WithCause(err))
		e, _ = apperr.First(err)
		entry = errTable[apperr.ErrInternal]
	}
	_ = c.Error(err)
	if c.Request.Context().Err() != nil {
		return
	}
	c.JSON(entry.status, ErrorResponse{Code: string(e.Code), Message: entry.message, RequestID: requestID})
}

// WriteInternal 供框架兜底与恢复路径共用，仍遵守取消和已提交响应的守卫。
func WriteInternal(c *gin.Context, requestID string) {
	Fail(c, apperr.ErrInternal.With("response.internal"), requestID)
	c.Abort()
}
