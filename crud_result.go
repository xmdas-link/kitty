package kitty

import jsoniter "github.com/json-iterator/go"

// CrudResult 结果
type CrudResult struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data,omitempty"`
	Count   *int        `json:"count,omitempty"`
	Message string      `json:"message,omitempty"`
	Ref     int64       `json:"ref,omitempty"`
}

// Result 。
type Result struct {
	CrudResult
	NameAs []*modelFieldAs `json:"-"`
	Cfg    jsoniter.API
}

// JsonAPI 可以由外部提供jsonapi
func (c *Result) JsonAPI(j jsoniter.API) {
	c.Cfg = j
}

// MarshalJSON ...
func (c *Result) MarshalJSON() ([]byte, error) {
	//c.Cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, nil})
	c.Cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, c.NameAs})
	jsoniter.RegisterTypeEncoder("time.Time", &timeAsString{})
	return c.Cfg.Marshal(c.CrudResult)
}
