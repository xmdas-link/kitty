package web

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/xmdas-link/kitty"
)

// Config 配置
type Config struct {
	Model  interface{}
	Ctx    externCtx
	DB     *gorm.DB
	RPC    kitty.RPC
	Callbk kitty.SuccessCallback
}

// NewLocalWeb ..
func NewLocalWeb(conf *Config) *CRUDWeb {
	res := kitty.NewResource(conf.Model)
	return &CRUDWeb{
		Crud: &kitty.LocalCrud{
			Model:  res.ModelName,
			DB:     conf.DB,
			Callbk: conf.Callbk,
		},
		Ctx: conf.Ctx,
	}
}

// NewRPCWeb .
func NewRPCWeb(conf *Config) *CRUDWeb {
	res := kitty.NewResource(conf.Model)
	return &CRUDWeb{
		Crud: &kitty.RPCCrud{
			Model: res.ModelName,
			RPC:   conf.RPC,
		},
		Ctx: conf.Ctx,
	}
}

type crudAction func() (interface{}, error)

// CRUDWeb web接口
type CRUDWeb struct {
	Crud kitty.CRUDInterface
	Ctx  externCtx
}

func (web *CRUDWeb) result(action crudAction, response webResponse) {
	res, err := action()
	if err != nil {
		response.fail(err)
	} else if res != nil {
		response.success(res)
	} else {
		response.success(gin.H{"code": 0, "message": "发生未知异常"})
	}
}
