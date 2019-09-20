package kitty

import (
	"reflect"

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
	types[strcase.ToSnake(s.Name())] = reflect2.TypeOf(v).(*reflect2.UnsafePtrType).Elem()

	typ := reflect2.TypeOf(v)
	structType := (typ.(*reflect2.UnsafePtrType)).Elem().(*reflect2.UnsafeStructType)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		type2 := field.Type()
		nativeType := DereferenceType(type2.Type1())
		if nativeType.Kind() == reflect.Slice {
			nativeType = nativeType.Elem()
			t1 := type2.(reflect2.SliceType).Elem()
			if t1.Kind() == reflect.Ptr {
				type2 = t1.(*reflect2.UnsafePtrType).Elem()
			}
		}
		if nativeType.Kind() == reflect.Ptr {
			nativeType = nativeType.Elem()
		}

		if nativeType.Kind() == reflect.Struct {
			if v, ok := type2.(*reflect2.UnsafePtrType); ok {
				type2 = v.Elem()
			}
			types[strcase.ToSnake(nativeType.Name())] = type2
		}
	}
}

// CreateModel 通过名称创建模型
func CreateModel(name string) *Structs {
	if v := types[strcase.ToSnake(name)]; v != nil {
		return createModelStructs(v.New())
	}
	panic("")
}
