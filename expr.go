package kitty

import (
	"errors"
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

	var setField = func(f *structs.Field, value interface{}) error {
		if err := f.Set(value); err != nil {
			return fmt.Errorf("%s: %s", f.Name(), err.Error())
		}
		return nil
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
		strfield := args[0].(string)
		if strings.Contains(strfield, ".") { //xxx.xx
			v := strings.Split(strfield, ".")
			field := e.s.Field(ToCamel(v[0]))
			if field.IsZero() {
				return nil, nil
			}
			field = field.Field(ToCamel(v[1]))
			if field.IsZero() {
				return nil, nil
			}
			value := field.Value()
			return nil, setField(e.f, value)
		}
		if f, ok := e.s.FieldOk(ToCamel(strfield)); ok {
			if !reflect.ValueOf(f.Value()).IsNil() {
				return DereferenceValue(reflect.ValueOf(f.Value())).Interface(), nil
			}
			return nil, nil
		}
		return nil, fmt.Errorf("$ unknown field %s", strfield)
	}
	functions["set"] = func(args ...interface{}) (interface{}, error) {
		return nil, e.s.SetFieldValue(e.f, args[0])
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
			return nil, s.SetFieldValue(e.f, v)
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
			return nil, setField(e.f, ss.raw)
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
				return nil, setField(e.f, ss.raw)
			}
		} else if tk.TypeOfField.Kind() == reflect.Slice {
			objValue := makeSlice(reflect.TypeOf(ss.raw), 0)
			result := objValue.Interface()
			if tx.Find(result).Error == nil {
				return nil, setField(e.f, reflect.ValueOf(result).Elem().Interface())
			}
		}

		return nil, nil
	}

	functions["batch_create"] = func(args ...interface{}) (interface{}, error) {
		s := e.s

		if len(args) > 0 {
			modelNameForCreate := strcase.ToSnake(e.f.Name())
			//count, _ := strconv.ParseInt(args[0].(string), 10, 64)
			count := args[0].(float64)
			slices := make([]*Structs, 0)
			for i := 0; i < int(count); i++ {
				screate := CreateModel(modelNameForCreate)
				slices = append(slices, screate)
			}

			for _, field := range s.Fields() {
				k := field.Tag("kitty")
				if len(k) > 0 && strings.Contains(k, fmt.Sprintf("param:%s", modelNameForCreate)) {
					tk := (&FormField{field}).TypeAndKind()
					if tk.KindOfField == reflect.Slice { // []*Strcuts []*int
						datavalue := field.Value() // slice
						dslice := reflect.ValueOf(datavalue)
						elemType := DereferenceType(tk.TypeOfField.Elem())
						if elemType.Kind() == reflect.Struct {
							for i := 0; i < dslice.Len(); i++ {
								screate := slices[i]
								dv := dslice.Index(i)
								ss := createModelStructs(dv.Interface())
								for _, field := range ss.Fields() {
									fname := field.Name()
									if f, ok := screate.FieldOk(fname); ok {
										if err := screate.SetFieldValue(f, field.Value()); err != nil {
											return nil, err
										}
									}
								}
							}
						} else {
							bindField := strings.Split(GetSub(k, "param"), ".")[1]
							for i := 0; i < len(slices); i++ {
								screate := slices[i]
								f := screate.Field(ToCamel(bindField))
								if err := screate.SetFieldValue(f, field.Value()); err != nil {
									return nil, err
								}
							}
						}
					} else if tk.KindOfField == reflect.Struct {
						for i := 0; i < len(slices); i++ {
							screate := slices[i]
							for _, subfield := range field.Fields() {
								fname := subfield.Name()
								if f, ok := screate.FieldOk(fname); ok {
									if err := screate.SetFieldValue(f, field.Value()); err != nil {
										return nil, err
									}
								}
							}
						}
					} else {
						for i := 0; i < len(slices); i++ {
							screate := slices[i]
							f := screate.Field(field.Name())
							if err := screate.SetFieldValue(f, field.Value()); err != nil {
								return nil, err
							}
						}
					}
				}
			}

			for i := 0; i < len(slices); i++ {
				screate := slices[i]
				crud := newcrud(&config{
					strs:   screate,
					search: &SearchCondition{},
					db:     e.db,
					ctx:    e.ctx,
				})
				if _, err := crud.createObj(); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}

		return nil, errors.New("param error in batch_create")
	}

	var f1 = func(field *structs.Field, args ...interface{}) (*Structs, error) {
		tk := (&FormField{field}).TypeAndKind()
		strs := CreateModel(tk.ModelName)

		params := strings.Split(args[0].(string), ",")
		for _, v := range params {
			param := strings.Split(v, "=") //like id=id_list ; id=1 ; count=10
			field := strs.Field(ToCamel(param[0]))
			if f, ok := e.s.FieldOk(ToCamel(param[1])); ok {
				if err := field.Set(f.Value()); err != nil {
					return nil, err
				}
			} else {
				if err := strs.SetFieldValue(field, param[1]); err != nil {
					return nil, err
				}
			}
		}
		return strs, nil
	}

	functions["qry"] = func(args ...interface{}) (interface{}, error) {

		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		res, err := newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		}).queryObj()

		if res != nil {
			return nil, setField(e.f, res)
		}
		return nil, err
	}

	functions["create"] = func(args ...interface{}) (interface{}, error) {
		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		res, err := newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		}).createObj()

		if res != nil {
			return nil, setField(e.f, res)
		}
		return nil, err
	}

	functions["update"] = func(args ...interface{}) (interface{}, error) {
		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		return nil, newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		}).updateObj()
	}

	functions["slice"] = func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			panic("")
		}
		//like slice('field_name', 2)
		//第一个参数，字段名称
		//第二个参数，索引
		strfield := args[0].(string)
		subfield := ""
		idx := int(args[1].(float64))
		if strings.Contains(strfield, ".") {
			v := strings.Split(strfield, ".")
			strfield = v[0]
			subfield = v[1]
		}
		field := e.s.Field(ToCamel(strfield))
		if field.IsZero() {
			return nil, fmt.Errorf("slice value is zero %s", field.Name())
		}

		fieldvalue := field.Value()
		slicevalue := DereferenceValue(reflect.ValueOf(fieldvalue))
		if slicevalue.Len() < idx {
			return nil, fmt.Errorf("slice idx overflow %s", field.Name())
		}

		value := slicevalue.Index(idx).Interface()
		if len(subfield) == 0 {
			return nil, setField(e.f, value)
		}
		strs := createModelStructs(value)
		return nil, setField(e.f, strs.Field(ToCamel(subfield)).Value())
	}
}

func (e *expr) eval(expString string) (interface{}, error) {
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(expString, e.functions)
	if err != nil {
		return nil, err
	}
	return expression.Evaluate(e.params)
}
