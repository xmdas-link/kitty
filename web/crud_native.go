package web

import (
	"errors"
	"net/http"

	"github.com/xmdas-link/kitty"
)

// List2 ..
func (web *CRUDWeb) List2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	c1 := &kitty.API{
		Form: r.Form,
		Crud: web.Crud,
		Ctx:  &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.List, c1.Ctx, &nativeResponse{W: w})
}

// One2 ...
func (web *CRUDWeb) One2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	c1 := &kitty.API{
		Form: r.Form,
		Crud: web.Crud,
		Ctx:  &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.One, c1.Ctx, &nativeResponse{W: w})
}

// Update2 ...
func (web *CRUDWeb) Update2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	c1 := &kitty.API{
		Form: r.PostForm,
		Crud: web.Crud,
		Ctx:  &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.Update, c1.Ctx, &nativeResponse{W: w})
}

// Create2 ...
func (web *CRUDWeb) Create2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if len(r.PostForm) == 0 {
		rsp := &nativeResponse{W: w}
		rsp.Response(&nativeCtx{c: r.Context(), ctx: web.Ctx}, nil, errors.New("nothing params"))
		return
	}

	c1 := &kitty.API{
		Form: r.PostForm,
		Crud: web.Crud,
		Ctx:  &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.Create, c1.Ctx, &nativeResponse{W: w})
}

// CallRPC2 ..
func (web *CRUDWeb) CallRPC2(w http.ResponseWriter, r *http.Request) {
	c1 := &kitty.API{
		Crud:   web.Crud,
		Params: web.Params,
		Ctx:    &nativeCtx{c: r.Context(), ctx: web.Ctx},
	}
	web.result(c1.CallRPC, c1.Ctx, &nativeResponse{W: w})
}
