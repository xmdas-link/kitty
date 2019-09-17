package web

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/kitty"
)

// List ...
func (web *WebCrud) List(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.Form,
		Crud:     web.Crud,
		Ctx:      &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.List, &ginResponse{C: c})
}

// One ...
func (web *WebCrud) One(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.Form,
		Crud:     web.Crud,
		Ctx:      &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.One, &ginResponse{C: c})
}

// Update ...
func (web *WebCrud) Update(c *gin.Context) {
	c.Request.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.PostForm,
		Crud:     web.Crud,
		Ctx:      &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.Update, &ginResponse{C: c})
}

// Create ...
func (web *WebCrud) Create(c *gin.Context) {
	c.Request.ParseForm()

	if len(c.Request.PostForm) == 0 {
		r := &ginResponse{C: c}
		r.fail(errors.New("nothing params"))
		return
	}

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     c.Request.PostForm,
		Crud:     web.Crud,
		Ctx:      &ginCtx{c: c, ctx: web.Ctx},
	}
	web.result(c1.Create, &ginResponse{C: c})
}
