package web

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/xmdas-link/kitty"
)

// NewWeb 创建对象
func NewWeb(m interface{}, path string, db *gorm.DB, ctx externCtx) *WebCrud {
	res := kitty.NewResource(m, path)
	return &WebCrud{
		Resource: res,
		Crud: &kitty.LocalCrud{
			Model: res.ModelName,
			DB:    db,
		},
		Ctx: ctx,
	}
}

type crudAction func() (interface{}, error)

// WebCrud web接口
type WebCrud struct {
	Resource *kitty.Resource
	Crud     kitty.CRUDInterface
	Ctx      externCtx
}

// RoutePath 路由名称
func (web *WebCrud) RoutePath() string {
	return web.Resource.RoutePath()
}

func (web *WebCrud) result(action crudAction, response webResponse) {
	res, err := action()
	if err != nil {
		response.fail(err)
	} else if res != nil {
		response.success(res)
	} else {
		response.success(gin.H{"code": 1, "message": "success"})
	}
}
