package web

import (
	"errors"
	"net/http"

	"github.com/xmdas-link/kitty"
)

// List2 ..
func (web *WebCrud) List2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     r.Form,
		Crud:     web.Crud,
		Ctx:      &nativeKittyCtx{r.Context()},
	}
	web.result(c1.List, &nativeResponse{W: w})
}

// One2 ...
func (web *WebCrud) One2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     r.Form,
		Crud:     web.Crud,
		Ctx:      &nativeKittyCtx{r.Context()},
	}
	web.result(c1.One, &nativeResponse{W: w})
}

// Update2 ...
func (web *WebCrud) Update2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     r.PostForm,
		Crud:     web.Crud,
		Ctx:      &nativeKittyCtx{r.Context()},
	}
	web.result(c1.Update, &nativeResponse{W: w})
}

// Create2 ...
func (web *WebCrud) Create2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if len(r.PostForm) == 0 {
		r := &nativeResponse{W: w}
		r.Fail(errors.New("nothing params"))
		return
	}

	c1 := &kitty.CRUD{
		Resource: web.Resource,
		Form:     r.PostForm,
		Crud:     web.Crud,
		Ctx:      &nativeKittyCtx{r.Context()},
	}
	web.result(c1.Create, &nativeResponse{W: w})
}
