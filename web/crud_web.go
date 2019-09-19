package web

import (
	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/kitty"
)

// Config 配置
type Config struct {
	Ctx      externCtx
	Crud     kitty.CRUDInterface
}

// NewWeb 创建对象
func NewWeb(conf *Config) *CRUDWeb {
	return &CRUDWeb{
		Crud:     conf.Crud,
		Ctx:      conf.Ctx,
	}
}

type crudAction func() (interface{}, error)

// CRUDWeb web接口
type CRUDWeb struct {
	Crud     kitty.CRUDInterface
	Ctx      externCtx
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
