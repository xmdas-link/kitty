package kitty

import (
	"fmt"
	"strings"
)

// Resource ....
type Resource struct {
	Model     interface{}
	ModelName string
	Strs      *Structs
}

// NewResource ...
func NewResource(m interface{}) *Resource {
	r := &Resource{
		Model: m,
		Strs:  CreateModelStructs(m),
	}
	RegisterType(m)
	r.ModelName = r.Strs.Name()
	r.checkValid()
	return r
}

func (res *Resource) checkValid() {
	for _, field := range res.Strs.Fields() {
		k := field.Tag("kitty")
		if strings.Contains(k, "param:") {
			modelfield := GetSub(k, "param")
			res.valid(modelfield)
		} else if strings.Contains(k, "bind:") {
			modelfield := GetSub(k, "bind")
			if modelfield != "bindresult" {
				res.valid(modelfield)
			}
		}
		functions := []string{
			"rds",
			"f",
			"current",
			"len",
			"sprintf",
			"default",
			"set",
			"rd_create",
			"rd_update",
			"qry",
			"create",
			"update",
			"vf",
			"count",
			"qry_if",
			"create_if",
			"update_if",
			"set_if",
			"rd_update_if",
			"rd_create_if",
		}
		for _, v := range functions {
			if strings.Contains(k, v+"(") && !strings.Contains(k, "runtime") && !strings.Contains(k, "getter") && !strings.Contains(k, "setter") {
				panic(fmt.Errorf("%s.%s param error: need getter,setter,runtime", res.ModelName, field.Name()))
			}
		}
	}
}

func test(s *Structs, mf, k string) {
	name := ToCamel(k)
	if _, ok := s.FieldOk(name); !ok {
		panic(fmt.Sprintf("kitty, param error :%s", mf))
	}
}

func (res *Resource) valid(modelfield string) {
	if !strings.Contains(modelfield, ".") {
		return
	}
	v := strings.Split(modelfield, ".")
	if v[0] == "$" || v[1] == "*" {
		return
	}
	if f, ok := res.Strs.FieldOk(v[0]); ok {
		test(TypeKind(f).Create(), modelfield, v[1])
		return
	}
	s := res.Strs.createModel(v[0])
	if vv := strings.Split(v[1], ","); len(vv) > 0 {
		for _, v1 := range vv {
			test(s, modelfield, v1)
		}
	} else {
		test(s, modelfield, v[1])
	}
}
