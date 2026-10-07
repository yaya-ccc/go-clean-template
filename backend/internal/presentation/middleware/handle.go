package middleware

import (
	"go-clean-template/backend/internal/kernel/apperr"
	"go-clean-template/backend/internal/presentation/response"

	"github.com/gin-gonic/gin"
)

// Bind 在注册处收拢 JSON 绑定、协议形状校验与渲染，使 handler 只处理协议逻辑。
// 请求 DTO 用指针接收者实现 Validate，内容业务规则由后续用例保证。
func Bind[T any, PT interface {
	*T
	Validate() error
}](h func(*gin.Context, PT) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Context().Err() != nil {
			return
		}
		var req T
		pt := PT(&req)
		if err := c.ShouldBindJSON(pt); err != nil {
			response.Fail(c, apperr.ErrInvalidInput.With("bind request body", apperr.WithCause(err)), RequestIDFrom(c))
			return
		}
		if err := pt.Validate(); err != nil {
			response.Fail(c, err, RequestIDFrom(c))
			return
		}
		resp, err := h(c, pt)
		render(c, resp, err)
	}
}

// Handle 为无 JSON 请求体的端点收拢调用与渲染，Cookie 与响应头仍由 handler 操作。
func Handle(h func(*gin.Context) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Context().Err() != nil {
			return
		}
		resp, err := h(c)
		render(c, resp, err)
	}
}

func render(c *gin.Context, resp any, err error) {
	if err != nil {
		response.Fail(c, err, RequestIDFrom(c))
		return
	}
	if resp == nil {
		response.Ack(c)
		return
	}
	response.OK(c, resp)
}
