package kitty

import (
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
)

// Resource ....
type Resource struct {
	Model     interface{}
	Prefix    string
	Path      string
	ModelName string
	Strs      *structs.Struct
}

// NewResource ...
func NewResource(m interface{}, path string) *Resource {
	r := &Resource{
		Model: m,
		Path:  path,
		Strs:  structs.New(m),
	}
	r.ModelName = r.Strs.Name()
	r.checkValid()
	return r
}

func (res *Resource) checkValid() {
	for _, field := range res.Strs.Fields() {
		k := field.Tag("kitty")
		if strings.Contains(k, "param:") {
			modelfield := GetSub(k, "param")
			valid(modelfield)
		} else if strings.Contains(k, "bind:") {
			modelfield := GetSub(k, "bind")
			if modelfield != "bindresult"{
				valid(modelfield)
			}
		}
	}
}

// RoutePath 路由
func (res *Resource) RoutePath() string {
	return strcase.ToSnake(res.Path)
}

func test(s *Structs, mf, k string) {
	name := ToCamel(k)
	if _, ok := s.FieldOk(name); !ok {
		panic(fmt.Sprintf("kitty, param error :%s", mf))
	}
}

func valid(modelfield string) {
	v := strings.Split(modelfield, ".")
	if v[0] == "$" || v[1] == "*" {
		return
	}
	s := NewModelStruct(v[0])
	if vv := strings.Split(v[1], ","); len(vv) > 0 {
		for _, v1 := range vv {
			test(s, modelfield, v1)
		}
	} else {
		test(s, modelfield, v[1])
	}
}
