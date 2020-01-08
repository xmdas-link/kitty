package web

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/xmdas-link/kitty"
)

// Config 配置
type Config struct {
	Model       interface{}
	Ctx         externCtx
	DB          *gorm.DB
	RPC         kitty.RPC
	Callbk      kitty.SuccessCallback
	Params      map[string]interface{}
	WebResponse WebResponse
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
		Ctx:         conf.Ctx,
		Params:      conf.Params,
		WebResponse: conf.WebResponse,
	}
}

// NewRPCWeb .
func NewRPCWeb(conf *Config) *CRUDWeb {
	kitty.NewResource(conf.Model)
	return &CRUDWeb{
		Crud: &kitty.RPCCrud{
			RPC: conf.RPC,
		},
		Ctx:         conf.Ctx,
		Params:      conf.Params,
		WebResponse: conf.WebResponse,
	}
}

type crudAction func() (interface{}, error)

// CRUDWeb web接口
type CRUDWeb struct {
	Crud        kitty.CRUDInterface
	Ctx         externCtx
	Params      map[string]interface{}
	WebResponse WebResponse
}

func (web *CRUDWeb) result(action crudAction, c kitty.Context, response WebResponse) {
	res, err := action()
	if web.WebResponse != nil {
		result := res.(*kitty.Result)
		web.WebResponse.Response(c, result.CrudResult.Data, err)
		return
	}
	if err != nil {
		response.Response(c, nil, err)
	} else if res != nil {
		response.Response(c, res, nil)
	} else {
		response.Response(c, gin.H{"code": 0, "message": "007"}, nil)
	}
}
