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
	if _, hasRegister := types[strcase.ToSnake(s.Name())]; hasRegister {
		return
	}
	types[strcase.ToSnake(s.Name())] = reflect2.TypeOf(v).(*reflect2.UnsafePtrType).Elem()

	var registerStructType = func(field reflect2.Type) {
		nativeType := DereferenceType(field.Type1())
		types[strcase.ToSnake(nativeType.Name())] = field
		RegisterType(field.New())
	}

	typ := reflect2.TypeOf(v)
	structType := (typ.(*reflect2.UnsafePtrType)).Elem().(*reflect2.UnsafeStructType)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.Type().Kind() == reflect.Struct {
			registerStructType(field.Type())
		} else if field.Type().Kind() == reflect.Ptr {
			ptrType := field.Type().(*reflect2.UnsafePtrType)
			if ptrType.Elem().Kind() == reflect.Struct {
				registerStructType(ptrType.Elem())
			}
		} else if field.Type().Kind() == reflect.Slice {
			sliceType := field.Type().(*reflect2.UnsafeSliceType)
			elemType := sliceType.Elem()
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.(*reflect2.UnsafePtrType).Elem()
			}
			if elemType.Kind() == reflect.Struct {
				registerStructType(elemType)
			}

		}
	}
}

// CreateModel 通过名称创建模型
func CreateModel(name string) *Structs {
	if v := types[strcase.ToSnake(name)]; v != nil {
		return CreateModelStructs(v.New())
	}
	panic("")
}
