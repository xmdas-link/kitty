package web

import (
	"context"

	"github.com/gin-gonic/gin"
)

// ExternCtx 提供由外部定义的，根据上下文获得当前uid的登录信息
type ExternCtx interface {
	GetUID(*gin.Context) string
	GetUIDFromContext(context.Context) string
}

type nativeKittyCtx struct {
	c   context.Context
	ctx ExternCtx
}

func (c *nativeKittyCtx) CurrentUID() string {
	return c.ctx.GetUIDFromContext(c.c)
}

type ginKittyCtx struct {
	c   *gin.Context
	ctx ExternCtx
}

func (c *ginKittyCtx) CurrentUID() string {
	//登录的信息存在gin的上下文。
	return c.ctx.GetUID(c.c)
}
