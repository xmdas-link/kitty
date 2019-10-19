package kitty

import (
	"net/url"
)

// API for web api
type API struct {
	Form   url.Values
	Crud   CRUDInterface
	Ctx    Context
	Params map[string]interface{}
}

// List 。
func (c *API) List() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "R", c.Ctx)
}

// One 单条记录
func (c *API) One() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "R", c.Ctx)
}

// Update ...
func (c *API) Update() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "U", c.Ctx)
}

// Create ...
func (c *API) Create() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.Crud.Do(s, "C", c.Ctx)
}

// CallRPC .
func (c *API) CallRPC() (interface{}, error) {
	s := &SearchCondition{Params: c.Params}
	return c.Crud.Do(s, "RPC", c.Ctx)
}
