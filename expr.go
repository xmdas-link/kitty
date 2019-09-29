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

	functions["len"] = func(args ...interface{}) (interface{}, error) {
		length := reflect.ValueOf(args[0]).Len()
		return (float64)(length), nil
	}
	functions["sprintf"] = func(args ...interface{}) (interface{}, error) {
		return fmt.Sprintf(args[0].(string), args[1:]...), nil
	}
	functions["default"] = func(args ...interface{}) (interface{}, error) {
		if e.f.IsZero() {
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
		if strings.Contains(strfield, ".") { //xxx.xx.x
			sliceIdx := 0
			if len(args) == 2 {
				sliceIdx = int(args[1].(float64))
			}
			return e.s.getValue(strfield, sliceIdx)
		}
		if f, ok := e.s.FieldOk(ToCamel(strfield)); ok && !f.IsZero() {
			fk := (&FormField{f}).TypeAndKind()
			if fk.KindOfField == reflect.Slice {
				thiskind := (&FormField{e.f}).TypeAndKind()
				if thiskind.KindOfField == reflect.Struct {
					if thiskind.ModelName != fk.ModelName {
						return nil, fmt.Errorf("model does not match %s", strfield)
					}
					idx := int(args[1].(float64))
					slicevalue := DereferenceValue(reflect.ValueOf(f.Value()))
					if slicevalue.Len() < idx {
						return nil, fmt.Errorf("slice idx overflow %s", f.Name())
					}
					return slicevalue.Index(idx).Interface(), nil
				}
			}
			return DereferenceValue(reflect.ValueOf(f.Value())).Interface(), nil
		}
		return nil, fmt.Errorf("$ unknown field %s", strfield)
	}
	functions["set"] = func(args ...interface{}) (interface{}, error) {
		return args[0], nil
	}
	functions["set_if"] = func(args ...interface{}) (interface{}, error) {
		if !args[0].(bool) {
			return nil, nil
		}
		return args[1], nil
	}

	functions["db"] = func(args ...interface{}) (interface{}, error) {
		s := e.s
		db := e.db
		argv := args[0].(string)

		//user.name.id=id
		//user.name.id=user.id
		v := strings.Split(argv, "=")

		v1 := strings.Split(v[0], ".")
		model := v1[0]
		fromfield := v1[1]
		v2 := []string{v1[2], v[1]}
		param := strings.Join(v2, "=")

		ss := CreateModel(model)
		if err := ss.fillValue(s, []string{param}); err != nil {
			return nil, err
		}

		tx := db.Select(fromfield)
		
		if ToCamel(v1[2])!="ID"{
			tx = tx.Where(ss.raw)
		}

		if !tx.First(ss.raw).RecordNotFound() {
			return ss.Field(ToCamel(fromfield)).Value(), nil
		}

		return nil, nil
	}

	// rd 单条记录 -> 模型
	functions["rd"] = func(args ...interface{}) (interface{}, error) {
		db := e.db
		argv := args[0].(string)
		v := strings.Split(argv, ",")

		model := (&FormField{e.f}).TypeAndKind().ModelName
		ss := CreateModel(model)
		if err := ss.fillValue(e.s, v); err != nil {
			return nil, err
		}
		if db.Where(ss.raw).First(ss.raw).Error == nil {
			return ss.raw, nil
		}
		return nil, nil
	}

	functions["rds"] = func(args ...interface{}) (interface{}, error) {
		s := e.s
		db := e.db
		tx := db

		tk := (&FormField{e.f}).TypeAndKind()
		model := tk.ModelName
		ss := CreateModel(model)

		if len(args) > 0 { // 参数查询 product_id = product.id
			argv := args[0].(string)
			v := strings.Split(argv, ",")
			if err := ss.fillValue(e.s, v); err != nil {
				return nil, err
			}
		}

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

		if tk.TypeOfField.Kind() == reflect.Struct {
			if tx.Where(ss.raw).First(ss.raw).Error == nil {
				return ss.raw, nil
			}
		} else if tk.TypeOfField.Kind() == reflect.Slice {
			objValue := makeSlice(reflect.TypeOf(ss.raw), 0)
			result := objValue.Interface()
			if tx.Where(ss.raw).Find(result).Error == nil {
				return reflect.ValueOf(result).Elem().Interface(), nil
			}
		}

		return nil, nil
	}

	var batchCreate = func(s *Structs, args ...interface{}) (interface{}, error) {
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
		return strs, strs.fillValue(e.s, params)
	}

	functions["qry"] = func(args ...interface{}) (interface{}, error) {
		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		return newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		}).queryObj()
	}

	functions["create_if"] = func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			panic("")
		}
		if !args[0].(bool) {
			return nil, nil
		}
		fun := functions["create"]
		return fun(args[1:]...)
	}

	functions["create"] = func(args ...interface{}) (interface{}, error) {
		tk := (&FormField{e.f}).TypeAndKind()
		if tk.KindOfField == reflect.Slice {
			return batchCreate(e.s, args...)
		}

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
			return strs.raw, nil
		}
		return nil, err
	}

	functions["update_if"] = func(args ...interface{}) (interface{}, error) {
		if len(args) < 2 {
			panic("")
		}
		if !args[0].(bool) {
			return nil, nil
		}
		fun := functions["update"]
		return fun(args[1:]...)
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
}

func (e *expr) eval(expString string) (interface{}, error) {
	if strings.Contains(expString, "create_if") || strings.Contains(expString, "update_if") || strings.Contains(expString, "set_if") {
		//create_if(result==1 && name==hello,'user_id=id,user_name=name')
		a1 := strings.Index(expString, "(")
		b1 := strings.Index(expString, ",")
		condition := expString[a1+1 : b1] // result==1 && name==hello
		key := []string{"&&", "==", "||", ">", ">=", "<", "<=", "!="}
		for _, v := range key {
			condition = strings.ReplaceAll(condition, v, ",")
		}
		key = strings.Split(condition, ",")
		for _, v := range key {
			fieldName := strings.TrimSpace(v)
			if len(fieldName) > 0 && e.params[fieldName] == nil {
				if strings.Contains(fieldName, ".") {
					if v, err := e.s.getValue(fieldName, 0); err == nil {
						str := strings.ReplaceAll(fieldName, ".", "_")
						expString = strings.ReplaceAll(expString, fieldName, str)
						e.params[str] = v
					}
				} else if f, ok := e.s.FieldOk(ToCamel(fieldName)); ok {
					e.params[fieldName] = DereferenceValue(reflect.ValueOf(f.Value())).Interface()
				}
			}
		}
	}

	expression, err := govaluate.NewEvaluableExpressionWithFunctions(expString, e.functions)
	if err != nil {
		return nil, err
	}
	return expression.Evaluate(e.params)
}

// Eval for test
func Eval(s *Structs, f *structs.Field, db *gorm.DB, exp string) (interface{}, error) {
	expr := &expr{
		db:        db,
		s:         s,
		f:         f,
		functions: make(map[string]govaluate.ExpressionFunction),
		params:    make(map[string]interface{}),
	}
	expr.params["s"] = s.raw
	expr.init()

	return expr.eval(exp)
}
