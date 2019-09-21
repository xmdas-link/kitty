package kitty

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"

	"github.com/Knetic/govaluate"
)

type expr struct {
	db        *gorm.DB
	s         *Structs
	f         *structs.Field
	functions map[string]govaluate.ExpressionFunction
	params    map[string]interface{}
	ctx       Context
}

func (e *expr) init() {
	functions := e.functions
	for k, v := range exprFuncs {
		functions[k] = v
	}

	functions["len"] = func(args ...interface{}) (interface{}, error) {
		length := reflect.ValueOf(args[0]).Len()
		return (float64)(length), nil
	}
	functions["sprintf"] = func(args ...interface{}) (interface{}, error) {
		return fmt.Sprintf(args[0].(string), args[1:]...), nil
	}
	functions["default"] = func(args ...interface{}) (interface{}, error) {
		if reflect.ValueOf(e.f.Value()).IsNil() {
			return args[0], nil
		}
		return nil, nil
	}

	functions["current"] = func(args ...interface{}) (interface{}, error) {
		s := args[0].(string)
		switch s {
		case "loginid":
			return e.ctx.CurrentUID(), nil
		}
		return nil, fmt.Errorf("current function: unexpert %s", s)
	}

	functions["f"] = func(args ...interface{}) (interface{}, error) {
		field := args[0].(string)
		if f, ok := e.s.FieldOk(ToCamel(field)); ok {
			if !reflect.ValueOf(f.Value()).IsNil() {
				return DereferenceValue(reflect.ValueOf(f.Value())).Interface(), nil
			}
			return nil, nil

		}
		return nil, fmt.Errorf("$ unknown field %s", field)
	}
	functions["set"] = func(args ...interface{}) (interface{}, error) {
		e.s.SetFieldValue(e.f, args[0])
		return nil, nil
	}
	functions["db"] = func(args ...interface{}) (interface{}, error) {
		s := e.s
		db := e.db
		argv := args[0].(string)

		v := strings.Split(argv, ".")
		model := v[0]
		fromfield := v[1]
		key := ToCamel(v[2])
		value := ToCamel(v[3])

		ss := CreateModel(model) //NewModelStruct(model)
		keyField := ss.Field(key)
		values := s.Field(value).Value()

		if !DereferenceValue(reflect.ValueOf(values)).IsValid() {
			return nil, fmt.Errorf("%s invalid value", value)
		}

		if err := ss.SetFieldValue(keyField, s.Field(value).Value()); err != nil {
			return nil, err
		}

		tx := db.Select(fromfield)
		if key != "ID" {
			tx = tx.Where(ss.raw)
		}

		if !tx.First(ss.raw).RecordNotFound() {
			v := ss.Field(ToCamel(fromfield)).Value()
			s.SetFieldValue(e.f, v)
		}

		return nil, nil
	}
	functions["rd"] = func(args ...interface{}) (interface{}, error) {
		s := e.s
		db := e.db
		argv := args[0].(string)

		v := strings.Split(argv, ".")
		keyField := v[0]
		valueField := ToCamel(v[1])

		model := (&FormField{e.f}).TypeAndKind().ModelName
		ss := CreateModel(model) //NewModelStruct(model)

		if db.Where(fmt.Sprintf("%s = ?", keyField), s.Field(valueField).Value()).First(ss.raw).Error == nil {
			e.f.Set(ss.raw)
		}

		return nil, nil
	}

	functions["rds"] = func(args ...interface{}) (interface{}, error) {
		s := e.s
		db := e.db
		tx := db

		if len(args) > 0 { // 参数查询 product_id = product.id
			argv := args[0].(string)
			if len(argv) > 0 {
				v := strings.Split(argv, "=")
				keyField := v[0]

				valueField := strings.Split(v[1], ".")
				//product.id
				field := s.Field(ToCamel(valueField[0])).Field(ToCamel(valueField[1]))
				tx = tx.Where(fmt.Sprintf("%s = ?", keyField), field.Value())
			}
		}

		tk := (&FormField{e.f}).TypeAndKind()
		model := tk.ModelName

		if ks := e.params["kittys"]; ks != nil {
			if kk, ok := ks.(*kittys); ok {
				if subqry := kk.subWhere(model); len(subqry) > 0 {
					for _, v := range subqry {
						tx = tx.Where(v.field, v.v...)
					}
				}
				j := kk.get(model)
				if j != nil && len(j.JoinTo) > 0 {
					joinTo := kk.get(j.JoinTo)
					ms := e.params["ms"].(*Structs)
					if fi, err := ms.GetRelationsWithModel(j.FieldName, joinTo.ModelName); err == nil {
						if fi.Relationship != "nothing" {
							associationKey := strcase.ToSnake(fi.ForeignKey)
							field := s.Field(joinTo.ModelName).Field(ToCamel(fi.AssociationForeignkey))
							tx = tx.Where(fmt.Sprintf("%s = ?", associationKey), field.Value())
						}
					}
				}
			}
		}

		ss := CreateModel(model) //NewModelStruct(model)

		if tk.TypeOfField.Kind() == reflect.Struct {
			if tx.First(ss.raw).Error == nil {
				e.f.Set(ss.raw)
			}
		} else if tk.TypeOfField.Kind() == reflect.Slice {
			objValue := makeSlice(reflect.TypeOf(ss.raw), 0)
			result := objValue.Interface()
			if tx.Find(result).Error == nil {
				e.f.Set(reflect.ValueOf(result).Elem().Interface())
			}
		}

		return nil, nil
	}
}

func (e *expr) eval(expString string) (interface{}, error) {
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(expString, e.functions)
	if err != nil {
		return nil, err
	}
	return expression.Evaluate(e.params)
}
