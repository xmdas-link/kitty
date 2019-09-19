package web

import (
	"errors"
	"net/http"

	"github.com/xmdas-link/kitty"
)

// List2 ..
func (web *CRUDWeb) List2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	c1 := &kitty.CRUD{
		Form:     r.Form,
		Crud:     web.Crud,
		Ctx:      &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.List, &nativeResponse{W: w})
}

// One2 ...
func (web *CRUDWeb) One2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	c1 := &kitty.CRUD{
		Form:     r.Form,
		Crud:     web.Crud,
		Ctx:      &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.One, &nativeResponse{W: w})
}

// Update2 ...
func (web *CRUDWeb) Update2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	c1 := &kitty.CRUD{
		Form:     r.PostForm,
		Crud:     web.Crud,
		Ctx:      &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.Update, &nativeResponse{W: w})
}

// Create2 ...
func (web *CRUDWeb) Create2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if len(r.PostForm) == 0 {
		r := &nativeResponse{W: w}
		r.fail(errors.New("nothing params"))
		return
	}

	c1 := &kitty.CRUD{
		Form:     r.PostForm,
		Crud:     web.Crud,
		Ctx:      &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.Create, &nativeResponse{W: w})
}
