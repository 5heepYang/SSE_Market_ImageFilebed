package middleware

import (
	"imagebed/response"
	"imagebed/service"

	"github.com/gin-gonic/gin"
)

// AnyAuth 同时支持 JWT（OAuth 会话）和 API Token 两种鉴权方式。
// 先用 JWT 解析，失败后再尝试 API Token；都失败才返回 401。
// 这样前端 Web 页面（JWT）和外部脚本（Token）都能调用同一接口。
func AnyAuth(auth *service.AuthService, tokens *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := BearerToken(c)
		if raw == "" {
			raw = c.Query("token")
		}
		if raw == "" {
			response.Unauthorized(c, "未登录")
			return
		}
		// 先试 JWT（Web 会话）
		if uid, err := auth.ParseToken(raw); err == nil {
			c.Set("user_id", uid)
			c.Next()
			return
		}
		// 再试 API Token（脚本/第三方）
		if t, err := tokens.Authenticate(raw); err == nil {
			c.Set("user_id", t.UserID)
			c.Next()
			return
		}
		response.Unauthorized(c, "Token 无效")
	}
}
