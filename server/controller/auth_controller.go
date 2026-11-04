package controller

import (
	"errors"
	"net/http"

	"imagebed/dto"
	"imagebed/middleware"
	"imagebed/response"
	"imagebed/service"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	svc    *service.AuthService
	unisso *service.UnissoService
}

func NewAuthController(svc *service.AuthService, unisso *service.UnissoService) *AuthController {
	return &AuthController{svc: svc, unisso: unisso}
}

// Login 邮箱+密码登录：UniSSO 校验（要求 internal=true）通过后签发本地会话 JWT。
func (c *AuthController) Login(ctx *gin.Context) {
	var req dto.LoginReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequest(ctx, "参数错误："+err.Error())
		return
	}
	if !c.unisso.IsConfigured() {
		response.Fail(ctx, http.StatusServiceUnavailable, 503, "登录服务未配置")
		return
	}
	info, err := c.unisso.Verify(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrNotInternal) {
			response.Fail(ctx, http.StatusForbidden, 403, err.Error())
			return
		}
		response.Fail(ctx, http.StatusUnauthorized, 401, err.Error())
		return
	}
	user, err := c.unisso.UpsertUser(info)
	if err != nil {
		response.ServerError(ctx, err.Error())
		return
	}
	token, err := c.unisso.FinishLogin(user)
	if err != nil {
		response.ServerError(ctx, err.Error())
		return
	}
	response.OK(ctx, dto.LoginResp{Token: token, User: user})
}

func (c *AuthController) Me(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	user, err := c.svc.GetUser(uid)
	if err != nil {
		response.Unauthorized(ctx, "用户不存在")
		return
	}
	response.OK(ctx, user)
}

// IssueCredential 为当前登录身份签发一张短期上传凭证（uid 即调用者自身）。
// 供主站等外部模块持有长效 token 后，换取一张可暴露给浏览器的短期 JWT 用于 tus 直连。
func (c *AuthController) IssueCredential(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	user, err := c.svc.GetUser(uid)
	if err != nil {
		response.Unauthorized(ctx, "用户不存在")
		return
	}
	token, expiresIn, err := c.svc.IssueUploadCredential(user)
	if err != nil {
		response.ServerError(ctx, err.Error())
		return
	}
	response.OK(ctx, dto.IssueCredentialResp{Token: token, ExpiresIn: expiresIn})
}
