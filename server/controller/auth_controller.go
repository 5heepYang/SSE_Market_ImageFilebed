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

// ssoCookieDomain 共享 Cookie 的父域。net/http 按 RFC 6265 去掉前导点，
// ssemarket.cn 与 .ssemarket.cn 语义等价（浏览器同样共享到全部子域）。
const ssoCookieDomain = "ssemarket.cn"

type AuthController struct {
	svc    *service.AuthService
	unisso *service.UnissoService
}

func NewAuthController(svc *service.AuthService, unisso *service.UnissoService) *AuthController {
	return &AuthController{svc: svc, unisso: unisso}
}

// Login 邮箱+密码登录：UniSSO 校验（要求 internal=true）通过后签发本地会话 JWT。
// 若 unisso 同时下发了共享会话，则按响应里的 sso_cookie_* 元数据种到父域。
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
	info, sso, err := c.unisso.Verify(req.Email, req.Password)
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
	if sso != nil && sso.Session != "" {
		ctx.SetSameSite(http.SameSiteLaxMode)
		ctx.SetCookie(sso.Name, sso.Session, sso.MaxAge, "/", sso.Domain, true, true)
	}
	response.OK(ctx, dto.LoginResp{Token: token, User: user})
}

// SSOTry 静默 SSO：读共享 Cookie（unisso_sid 优先，兼容期回落 sse_sso），
// 向 UniSSO 校验会话后 upsert 用户并签发本地 JWT；响应结构与登录同构，不重复种 Cookie。
func (c *AuthController) SSOTry(ctx *gin.Context) {
	session := ssoSessionFromCookie(ctx)
	if session == "" {
		response.Fail(ctx, http.StatusUnauthorized, 401, "no_sso")
		return
	}
	if !c.unisso.IsConfigured() {
		response.Fail(ctx, http.StatusServiceUnavailable, 503, "sso_unavailable")
		return
	}
	info, err := c.unisso.VerifySession(session)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidSSO):
			response.Fail(ctx, http.StatusUnauthorized, 401, "invalid_sso")
		case errors.Is(err, service.ErrNotInternal):
			response.Fail(ctx, http.StatusUnauthorized, 401, "not_internal")
		default:
			response.Fail(ctx, http.StatusServiceUnavailable, 503, "sso_unavailable")
		}
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

// Logout 单点登出：撤销共享会话 + 清父域 Cookie（新旧名双清）。
// 本地 JWT 无服务端状态，浏览器删除即失效。
func (c *AuthController) Logout(ctx *gin.Context) {
	if session := ssoSessionFromCookie(ctx); session != "" && c.unisso.IsConfigured() {
		c.unisso.RevokeSession(session)
	}
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(service.SSOCookieName, "", -1, "/", ssoCookieDomain, true, true)
	ctx.SetCookie(service.SSOCookieLegacyName, "", -1, "/", ssoCookieDomain, true, true)
	response.OK(ctx, nil)
}

// ssoSessionFromCookie 双读共享 SSO Cookie：新名 unisso_sid 优先，兼容期回落旧名 sse_sso。
func ssoSessionFromCookie(ctx *gin.Context) string {
	if v, err := ctx.Cookie(service.SSOCookieName); err == nil && v != "" {
		return v
	}
	if v, err := ctx.Cookie(service.SSOCookieLegacyName); err == nil && v != "" {
		return v
	}
	return ""
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
