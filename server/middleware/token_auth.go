package middleware

import (
	"imagebed/response"
	"imagebed/service"

	"github.com/gin-gonic/gin"
)

// TokenAuth API 上传鉴权：校验个人 API Token（支持 Authorization 头或 ?token= 查询参数）。
func TokenAuth(tokens *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := BearerToken(c)
		if raw == "" {
			raw = c.Query("token")
		}
		t, err := tokens.Authenticate(raw)
		if err != nil {
			response.Unauthorized(c, err.Error())
			return
		}
		c.Set("user_id", t.UserID)
		c.Next()
	}
}
