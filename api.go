package kitty

import (
	"net/url"
	"strconv"
)

// CRUD for web api
type CRUD struct {
	Resource *Resource
	Form     url.Values
	Crud     CRUDInterface
	Ctx      context
}

// List 。
func (c *CRUD) List() (interface{}, error) {
	Page := &Page{}
	if page := c.Form["page"]; len(page) > 0 {
		p, _ := strconv.ParseUint(page[0], 10, 64)
		Page.Page = uint32(p)
		delete(c.Form, "page")
	}
	if limit := c.Form["limit"]; len(limit) > 0 {
		l, _ := strconv.ParseUint(limit[0], 10, 64)
		Page.Limit = uint32(l)
		delete(c.Form, "limit")
	}

	s := &SearchCondition{FormValues: c.Form}
	if Page.Limit > 0 && Page.Page > 0 {
		s.Page = Page
	}
	return c.Crud.Do(s, "R", c.Ctx)
}

// One 单条记录
func (c *CRUD) One() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "R", c.Ctx)
}

// Update ...
func (c *CRUD) Update() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "U", c.Ctx)
}

// Create ...
func (c *CRUD) Create() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "C", c.Ctx)
}
