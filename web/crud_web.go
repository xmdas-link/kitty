package web

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/xmdas-link/kitty"
)

// Config 配置
type Config struct {
	Model interface{}
	Path  string
	DB    *gorm.DB
	Ctx   externCtx
}

// NewWeb 创建对象
func NewWeb(conf *Config) *CRUDWeb {
	res := kitty.NewResource(conf.Model, conf.Path)
	return &CRUDWeb{
		Resource: res,
		Crud: &kitty.LocalCrud{
			Model: res.ModelName,
			DB:    conf.DB,
		},
		Ctx: conf.Ctx,
	}
}

type crudAction func() (interface{}, error)

// CRUDWeb web接口
type CRUDWeb struct {
	Resource *kitty.Resource
	Crud     kitty.CRUDInterface
	Ctx      externCtx
}

// RoutePath 路由名称
func (web *CRUDWeb) RoutePath() string {
	return web.Resource.RoutePath()
}

func (web *CRUDWeb) result(action crudAction, response webResponse) {
	res, err := action()
	if err != nil {
		response.fail(err)
	} else if res != nil {
		response.success(res)
	} else {
		response.success(gin.H{"code": 1, "message": "success"})
	}
}
