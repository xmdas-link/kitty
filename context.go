package kitty

import "context"

// Context 获取登录信息
type Context interface {
	CurrentUID() (string, error)
	GetCtxInfo(string) (string, error)
	GetCtx() context.Context
}
