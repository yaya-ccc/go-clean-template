package middleware

import "github.com/gin-gonic/gin"

// 请求作用域值的读写收拢于此；应用层日志收集器仍使用标准 context。
const (
	requestIDKey = "request_id"
	userIDKey    = "auth_user_id"
)

func withRequestID(c *gin.Context, id string) { c.Set(requestIDKey, id) }

// RequestIDFrom 为错误渲染与诊断提供同源标识，response 包不自行查找请求值。
func RequestIDFrom(c *gin.Context) string { return c.GetString(requestIDKey) }

func withUserID(c *gin.Context, id int64) { c.Set(userIDKey, id) }

// CurrentUserID 只读取 RequireAuth 已确认的身份，不采用客户端自报标识。
func CurrentUserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(userIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}
