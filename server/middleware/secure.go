package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders 为所有响应添加安全头，降低 XSS、点击劫持、MIME 嗅探等风险。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "0")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// HSTS：仅生产域名；本地调试不开启
		if c.Request.Host != "localhost" && c.Request.Host != "127.0.0.1" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
