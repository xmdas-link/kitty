package kitty

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
	"github.com/modern-go/reflect2"
)

var types map[string]reflect2.Type

// RegisterType 注册模型
func RegisterType(v interface{}) {
	if types == nil {
		types = make(map[string]reflect2.Type)
	}
	s := &Structs{structs.New(v), v}
	if _, hasRegister := types[strcase.ToSnake(s.Name())]; hasRegister {
		return
	}
	types[strcase.ToSnake(s.Name())] = reflect2.TypeOf(v).(*reflect2.UnsafePtrType).Elem()
}

// CreateModel 通过名称创建模型
func CreateModel(name string) *Structs {
	if v := types[strcase.ToSnake(name)]; v != nil {
		return CreateModelStructs(v.New())
	}
	panic(fmt.Sprintf("model: %s must be registered.", name))
}
