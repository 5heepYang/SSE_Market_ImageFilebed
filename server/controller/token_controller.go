package controller

import (
	"strconv"

	"imagebed/dto"
	"imagebed/middleware"
	"imagebed/response"
	"imagebed/service"

	"github.com/gin-gonic/gin"
)

type TokenController struct {
	svc *service.TokenService
}

func NewTokenController(svc *service.TokenService) *TokenController {
	return &TokenController{svc: svc}
}

func (c *TokenController) Create(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	var req dto.CreateTokenReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequest(ctx, "参数错误："+err.Error())
		return
	}
	raw, t, err := c.svc.Create(uid, req.Name, req.ExpiresAt)
	if err != nil {
		response.ServerError(ctx, err.Error())
		return
	}
	response.OK(ctx, dto.CreateTokenResp{
		Token:     raw,
		ID:        t.ID,
		Name:      t.Name,
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
	})
}

func (c *TokenController) List(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	tokens, err := c.svc.List(uid)
	if err != nil {
		response.ServerError(ctx, err.Error())
		return
	}
	response.OK(ctx, tokens)
}

func (c *TokenController) Revoke(ctx *gin.Context) {
	uid := middleware.UserID(ctx)
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ctx, "非法 ID")
		return
	}
	if err := c.svc.Revoke(uid, uint(id)); err != nil {
		response.ServerError(ctx, err.Error())
		return
	}
	response.OK(ctx, nil)
}
