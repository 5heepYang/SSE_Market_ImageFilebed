package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构：code 为 0 表示成功。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

// OKWithMessage 成功但需要自定义提示信息（如部分成功）。
func OKWithMessage(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: message, Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, message string) {
	c.AbortWithStatusJSON(httpStatus, Response{Code: code, Message: message})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, 400, message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, 401, message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, 403, message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, 404, message)
}

func ServerError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, 500, message)
}
