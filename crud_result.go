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
	NameAs map[string][]string `json:"-"`
	Cfg    jsoniter.API
}

// JsonAPI 可以由外部提供jsonapi
func (c *Result) JsonAPI(j jsoniter.API) {
	c.Cfg = j
}

// MarshalJSON ...
func (c *Result) MarshalJSON() ([]byte, error) {
	c.Cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, []string{}, ""})
	for k, v := range c.NameAs {
		c.Cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, v, k})
	}
	jsoniter.RegisterTypeEncoder("time.Time", &TimeAsString{})
	return c.Cfg.Marshal(c.CrudResult)
}
