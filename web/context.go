package web

import (
	"context"

	"github.com/gin-gonic/gin"
)

// externCtx 提供由外部定义的，根据上下文获得当前uid的登录信息
type externCtx interface {
	GetUID(interface{}) string
}

// nativeCtx 原生上下文
type nativeCtx struct {
	c   context.Context
	ctx externCtx
}

func (c *nativeCtx) CurrentUID() string {
	return c.ctx.GetUID(c.c)
}

type ginCtx struct {
	c   *gin.Context
	ctx externCtx
}

func (c *ginCtx) CurrentUID() string {
	//登录的信息存在gin的上下文。
	return c.ctx.GetUID(c.c)
}
