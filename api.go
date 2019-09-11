package kitty

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"

	"github.com/jinzhu/gorm"
)

// CRUD for web api
type CRUD struct {
	Resource *Resource
	Form     url.Values
	DB       *gorm.DB
	Ctx      *gin.Context
}

// CrudResult 结果
type CrudResult struct {
	Code  int         `json:"code,omitempty"`
	Data  interface{} `json:"data,omitempty"`
	Page  *Page       `json:"page,omitempty"`
	Count *int        `json:"count,omitempty"`
}

type Result struct {
	CrudResult
	NameAs map[string][]string `json:"-"`
}

// MarshalJSON ...
func (c Result) MarshalJSON() ([]byte, error) {
	cfg := jsoniter.Config{}.Froze()
	cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, []string{}, ""})
	for k, v := range c.NameAs {
		cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, v, k})
	}
	jsoniter.RegisterTypeEncoder("time.Time", &timeAsString{})
	return cfg.Marshal(c.CrudResult)
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
	return c.do(s, "R", true)
}

// One 单条记录
func (c *CRUD) One() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.do(s, "R", false)
}

// Update ...
func (c *CRUD) Update() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.do(s, "U", false)
}

// Create ...
func (c *CRUD) Create() (interface{}, error) {
	s := &SearchCondition{FormValues: c.Form}
	return c.do(s, "C", false)
}

func (c *CRUD) do(search *SearchCondition, action string, multi bool) (interface{}, error) {

	if err := search.CheckParamValid(c.Resource.ModelName); err != nil {
		return nil, err
	}

	s := NewModelStruct(c.Resource.ModelName)
	if s == nil {
		return nil, errors.New("error in create model")
	}
	if err := s.ParseFormValues(search.FormValues); err != nil {
		return nil, err
	}

	var (
		res interface{}
		err error
	)

	switch action {
	case "C":
		res, err = createObj(s, search, c.DB, c.Ctx)
	case "R":
		res, err = queryObj(s, search, c.DB, c.Ctx)
	case "U":
		err = updateObj(s, search, c.DB, c.Ctx)
	default:
		return nil, errors.New("unknown model action")
	}
	if res != nil {
		result := CrudResult{
			Code: 1,
			Data: res,
		}
		NameAs := make(map[string][]string)
		s.nameAs(NameAs)
		if search.Page != nil {
			result.Page = search.Page
			result.Count = new(int)
			*result.Count = search.ReturnCount
		}
		return Result{
			result,
			NameAs,
		}, nil
	}
	return nil, err
}
