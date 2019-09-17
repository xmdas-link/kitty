package web

import (
	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/kitty"
)

// WebCrud web接口
type WebCrud struct {
	Resource *kitty.Resource
	Crud     kitty.CRUDInterface
}

func (web *WebCrud) RoutePath() string {
	return web.Resource.RoutePath()
}

type crudAction func() (interface{}, error)

func (web *WebCrud) result(action crudAction, resp webResponse) {
	res, err := action()
	if err != nil {
		resp.Fail(err)
	} else if res != nil {
		resp.Success(res)
	} else {
		resp.Success(gin.H{"code": 1})
	}
}
