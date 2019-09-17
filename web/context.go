package web

import (
	"context"

	"github.com/gin-gonic/gin"
)

type nativeKittyCtx struct {
	ctx context.Context
}

func (c *nativeKittyCtx) CurrentUID() string {
	//登录的信息存在gin的上下文。
	return ""
}

type ginKittyCtx struct {
	*gin.Context
}

func (c *ginKittyCtx) CurrentUID() string {
	//登录的信息存在gin的上下文。
	user := c.GetStringMapString("AuthUser")
	if uid, ok := user["id"]; ok {
		return uid
	}
	return ""
}
