package web

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/kitty"
)

// List ...
func (web *CRUDWeb) List(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.API{
		Form: c.Request.Form,
		Crud: web.Crud,
		Ctx:  &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.List, c1.Ctx, &ginResponse{C: c})
}

// One ...
func (web *CRUDWeb) One(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.API{
		Form: c.Request.Form,
		Crud: web.Crud,
		Ctx:  &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.One, c1.Ctx, &ginResponse{C: c})
}

// Update ...
func (web *CRUDWeb) Update(c *gin.Context) {
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		c.Request.ParseMultipartForm(32 << 20)
	} else {
		c.Request.ParseForm()
	}

	c1 := &kitty.API{
		Form: c.Request.PostForm,
		Crud: web.Crud,
		Ctx:  &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.Update, c1.Ctx, &ginResponse{C: c})
}

// Create ...
func (web *CRUDWeb) Create(c *gin.Context) {
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		c.Request.ParseMultipartForm(32 << 20)
	} else {
		c.Request.ParseForm()
	}

	if len(c.Request.PostForm) == 0 {
		r := &ginResponse{C: c}
		r.Response(&ginCtx{c: c, ctx: web.Ctx}, nil, errors.New("nothing params"))
		return
	}

	c1 := &kitty.API{
		Form: c.Request.PostForm,
		Crud: web.Crud,
		Ctx:  &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.Create, c1.Ctx, &ginResponse{C: c})
}

// CallRPC ..
func (web *CRUDWeb) CallRPC(c *gin.Context) {
	form := c.Request.Form
	if c.Request.Method == "POST" {
		form = c.Request.PostForm
	}
	c1 := &kitty.API{
		Form:   form,
		Crud:   web.Crud,
		Params: web.Params,
		Ctx:    &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.CallRPC, c1.Ctx, &ginResponse{C: c})
}
