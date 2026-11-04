package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BearerToken 从 Authorization 头提取 Bearer Token（无 Bearer 前缀时原样返回）。
func BearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return strings.TrimSpace(h)
}

// UserID 取出鉴权中间件注入的用户 ID。
func UserID(c *gin.Context) uint {
	v, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	uid, _ := v.(uint)
	return uid
}
// MaxBody 限制请求体读取上限，防止超大请求耗尽内存。
func MaxBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
