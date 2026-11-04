package middleware

import (
	"imagebed/response"
	"imagebed/service"

	"github.com/gin-gonic/gin"
)

// JWTAuth Web 会话鉴权：校验 JWT 并把 user_id 注入上下文。
func JWTAuth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := BearerToken(c)
		if raw == "" {
			response.Unauthorized(c, "未登录")
			return
		}
		uid, err := auth.ParseToken(raw)
		if err != nil {
			response.Unauthorized(c, err.Error())
			return
		}
		c.Set("user_id", uid)
		c.Next()
	}
}
