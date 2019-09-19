package kitty

import (
	"github.com/iancoleman/strcase"
	"github.com/modern-go/reflect2"
)

var types map[string]reflect2.Type

// RegisterType 注册模型
func RegisterType(m interface{}) {
	if types == nil {
		types = make(map[string]reflect2.Type)
	}
	s := createModelStructs(m)
	types[strcase.ToSnake(s.Name())] = reflect2.TypeOf(m).(*reflect2.UnsafePtrType).Elem()
}

// CreateModel 通过名称创建模型
func CreateModel(name string) *Structs {
	if v := types[strcase.ToSnake(name)]; v != nil {
		return createModelStructs(v.New())
	}
	panic("")
}
