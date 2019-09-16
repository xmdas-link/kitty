package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/filter"
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

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, filter.H{Ctx: c, Data: data})
	//	c.JSON(http.StatusOK, data)
}
func fail(c *gin.Context, err error) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": err.Error()})
}

// List ...
func (web *WebCrud) List(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.Form,
		Crud:     web.Crud,
		Ctx:      &ginKittyCtx{c},
	}
	res, err := c1.List()
	if err != nil {
		fmt.Println(err)
		fail(c, err)
	} else if res != nil {
		success(c, res)
	} else {
		success(c, "")
	}
}

// One ...
func (web *WebCrud) One(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.Form,
		Crud:     web.Crud,
		Ctx:      &ginKittyCtx{c},
	}
	res, err := c1.One()
	if err != nil {
		fmt.Println(err)
		fail(c, err)
	} else if res != nil {
		success(c, res)
	} else {
		success(c, "")
	}
}

// Update ...
func (web *WebCrud) Update(c *gin.Context) {
	c.Request.ParseForm()
	//	if len(c.Request.PostForm) == 0 {
	//		c.JSON(200, "invalid request.")
	//		return
	//	}

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.PostForm,
		Crud:     web.Crud,
		Ctx:      &ginKittyCtx{c},
	}
	res, err := c1.Update()
	if err != nil {
		//fmt.Println(err)
		fail(c, err)
	} else if res != nil {
		success(c, res)
	} else {
		success(c, gin.H{"code": 1})
	}
}

// Create ...
func (web *WebCrud) Create(c *gin.Context) {
	c.Request.ParseForm()

	if len(c.Request.PostForm) == 0 {
		//	ctx.FailWithCode(api.CODE_ERROR_DEFAULT, fmt.Sprintf("%s create error. nothing params", web.Resource.ModelName))
		return
	}

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.PostForm,
		Crud:     web.Crud,
		Ctx:      &ginKittyCtx{c},
	}
	res, err := c1.Create()
	if err != nil {
		fmt.Println(err)
		fail(c, err)
	} else if res != nil {
		success(c, res)
	} else {
		success(c, gin.H{"code": 1})
	}
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
