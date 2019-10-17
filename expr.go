package kitty

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
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
	e.params["nil"] = nil

	functions := e.functions
	for k, v := range exprFuncs {
		functions[k] = v
	}

	var batchfill = func(s *Structs, f *structs.Field, args ...interface{}) (interface{}, error) {
		count := float64(0)
		if len(args) > 0 {
			count = args[0].(float64)
		}
		tk := TypeKind(f)
		modelNameForCreate := strcase.ToSnake(tk.ModelName)
		slices := make([]*Structs, 0)
		if count > 0 {
			for i := 0; i < int(count); i++ {
				screate := tk.create()
				slices = append(slices, screate)
			}
		} else {
			// 并不需要动态创建，则当前的字段是有值的。
			// 以当前的字段创建
			fvalue := f.Value()
			dslice := reflect.ValueOf(fvalue)
			for i := 0; i < dslice.Len(); i++ {
				screate := CreateModelStructs(dslice.Index(i).Interface())
				slices = append(slices, screate)
			}
		}

		for _, field := range s.Fields() {
			k := field.Tag("kitty")
			if len(k) > 0 && strings.Contains(k, fmt.Sprintf("param:%s", modelNameForCreate)) {
				tk := TypeKind(field)
				if tk.KindOfField == reflect.Slice { // []*Strcuts []*int
					datavalue := field.Value() // slice
					dslice := reflect.ValueOf(datavalue)
					elemType := DereferenceType(tk.TypeOfField.Elem())
					if elemType.Kind() == reflect.Struct {
						for i := 0; i < dslice.Len(); i++ {
							screate := slices[i]
							dv := dslice.Index(i)
							ss := CreateModelStructs(dv.Interface())
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
					bindField := strings.Split(GetSub(k, "param"), ".")[1]
					for i := 0; i < len(slices); i++ {
						screate := slices[i]
						f := screate.Field(ToCamel(bindField))
						if err := screate.SetFieldValue(f, field.Value()); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		return slices, nil
	}

	functions["len"] = func(args ...interface{}) (interface{}, error) {
		if args == nil {
			return (float64)(0), nil
		}
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
			return e.ctx.CurrentUID()
		default:
			return e.ctx.GetCtxInfo(s)
		}
	}

	functions["f"] = func(args ...interface{}) (interface{}, error) {
		strfield := args[0].(string)
		if strings.Contains(strfield, ".") { //xxx.xx.x or xxx[0].xx.x
			return e.s.getValue(strfield)
		}
		sliceIdx := -1
		if i := strings.Index(strfield, "["); i > 0 {
			b := strings.Index(strfield, "]")
			str := strfield[i+1 : b]
			strfield = strfield[:i]
			idx, _ := strconv.ParseInt(str, 10, 64)
			sliceIdx = int(idx)
		}
		if f, ok := e.s.FieldOk(ToCamel(strfield)); ok {
			if f.IsZero() {
				return nil, nil
			}
			fk := TypeKind(f)
			if fk.KindOfField == reflect.Slice {
				thiskind := TypeKind(e.f)
				if thiskind.KindOfField == reflect.Struct {
					if thiskind.ModelName != fk.ModelName {
						return nil, fmt.Errorf("model does not match %s", strfield)
					}
					slicevalue := DereferenceValue(reflect.ValueOf(f.Value()))
					if slicevalue.Len() < sliceIdx {
						return nil, fmt.Errorf("slice idx overflow %s", f.Name())
					}
					return slicevalue.Index(sliceIdx).Interface(), nil
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

		ss := s.createModel(model)
		if err := ss.fillValue(s, []string{param}); err != nil {
			return nil, err
		}

		tx := db.Select(fromfield)

		if ToCamel(v1[2]) != "ID" {
			tx = tx.Where(ss.raw)
		}

		if !tx.First(ss.raw).RecordNotFound() {
			return ss.Field(ToCamel(fromfield)).Value(), nil
		}

		return nil, nil
	}

	//创建单条记录
	functions["rd_create_if"] = func(args ...interface{}) (interface{}, error) {
		if !args[0].(bool) {
			return nil, nil
		}
		fun := functions["rd_create"]
		return fun(args[1:]...)
	}
	//更新单条记录
	functions["rd_update_if"] = func(args ...interface{}) (interface{}, error) {
		if !args[0].(bool) {
			return nil, nil
		}
		fun := functions["rd_update"]
		return fun(args[1:]...)
	}
	var rdBatchCreate = func(s *Structs, f *structs.Field, args ...interface{}) (interface{}, error) {
		value, err := batchfill(s, f, args...)
		if err != nil {
			return nil, err
		}
		slices := value.([]*Structs)
		for i := 0; i < len(slices); i++ {
			screate := slices[i]
			if err := e.db.New().Create(screate.raw).Error; err != nil {
				return nil, err
			}
		}
		if len(slices) > 0 {
			objSlice := makeSlice(reflect.TypeOf(slices[0].raw), len(slices))
			objValue := objSlice.Elem()
			for i := 0; i < len(slices); i++ {
				screate := slices[i]
				objValue.Index(i).Set(reflect.ValueOf(screate.raw))
			}
			return objValue.Interface(), nil
		}
		return nil, nil
	}
	//创建单条记录
	functions["rd_create"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField == reflect.Slice {
			return rdBatchCreate(e.s, e.f, args...)
		}
		db := e.db
		argv := args[0].(string)
		v := strings.Split(argv, ",")
		ss := tk.create()
		if err := ss.fillValue(e.s, v); err != nil {
			return nil, err
		}

		err := db.Create(ss.raw).Error
		if err == nil {
			return ss.raw, nil
		}
		return nil, err
	}

	//更新单条记录  格式  update:xx=xx, where: xx=xx
	functions["rd_update"] = func(args ...interface{}) (interface{}, error) {
		tx := e.db
		updateCondition := args[0].(string)
		whereCondition := args[1].(string)

		tk := TypeKind(e.f)
		sUpdate := tk.create()
		tx = tx.Model(sUpdate.raw)

		vWhere := strings.Split(whereCondition, ",")
		for _, expression := range vWhere {
			operators := []string{" LIKE ", "<>", ">=", "<=", ">", "<", "="}

			for _, oper := range operators {
				if strings.Contains(expression, oper) {
					vv := strings.Split(expression, oper)
					param := trimSpace(vv[1])
					res, err := e.s.getValue(param)
					if err != nil {
						return nil, err
					}
					fname := strcase.ToSnake(trimSpace(vv[0]))
					tx = tx.Where(fmt.Sprintf("%s %s ?", fname, oper), res)
					break
				}
			}
		}

		updates := make(map[string]interface{})

		vUpdate := strings.Split(updateCondition, ",")
		for _, expression := range vUpdate {
			if strings.Contains(expression, "=") {
				vv := strings.Split(expression, "=")
				param := trimSpace(vv[1])
				res, err := e.s.getValue(param)
				if err != nil {
					return nil, err
				}
				fname := strcase.ToSnake(trimSpace(vv[0]))
				updates[fname] = res
			}
		}

		if err := tx.Updates(updates).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}

	// rds:  rds('key=value','user.field,field') 第二项不是必填项。
	// 当kitty字段不是gorm的时候，需声明第二项
	functions["rds"] = func(args ...interface{}) (interface{}, error) {
		s := e.s
		tx := e.db
		tk := TypeKind(e.f)
		model := tk.ModelName
		modelDeclared := false
		fieldSel := ""

		if len(args) == 2 {
			fieldSel = args[1].(string)
			if strings.Contains(fieldSel, ".") {
				v := strings.Split(fieldSel, ".")
				model = v[0]
				fieldSel = v[1]
				modelDeclared = true
			}
			if len(fieldSel) > 0 {
				tx = tx.Select(fieldSel)
			}
		}

		var ss *Structs
		if modelDeclared {
			ss = s.createModel(model)
		} else {
			ss = tk.create()
		}

		if len(args) > 0 { // 参数查询 product_id = product.id
			argv := args[0].(string)
			if len(argv) > 0 {
				v := strings.Split(argv, ",")
				if len(v) > 0 {
					for _, expression := range v {
						operators := []string{" LIKE ", "<>", ">=", "<=", ">", "<", "="}

						for _, oper := range operators {
							if strings.Contains(expression, oper) {
								vv := strings.Split(expression, oper)
								param := trimSpace(vv[1])
								res, err := s.getValue(param)
								if err != nil {
									return nil, err
								}
								fname := strcase.ToSnake(trimSpace(vv[0]))
								tx = tx.Where(fmt.Sprintf("%s %s ?", fname, oper), res)
								break
							}
						}
					}
				}
			}
		}

		if ks := e.params["kittys"]; ks != nil {
			if kk, ok := ks.(*kittys); ok {
				if subqry := kk.subWhere(model); len(subqry) > 0 {
					for _, v := range subqry {
						tx = tx.Where(v.whereExpr(), v.value...)
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
		//	tx = tx.Where(ss.raw)

		var err error
		var res interface{}

		switch tk.TypeOfField.Kind() {
		case reflect.Struct:
			if len(args) == 2 && modelDeclared {
				tx = tx.Model(ss.raw)
				result := tk.create()
				err = tx.Scan(result.raw).Error
				res = result.raw
			} else {
				err = tx.First(ss.raw).Error
				res = ss.raw
			}
			if err != nil {
				return nil, nil
			}
		case reflect.Slice: // like []UserResult []string
			if len(args) == 2 && modelDeclared {
				tx = tx.Model(ss.raw)
				rt := DereferenceType(tk.TypeOfField.Elem())
				if rt.Kind() == reflect.Struct {
					result := tk.create()
					objValue := makeSlice(reflect.TypeOf(result.raw), 0)
					err = tx.Scan(objValue.Interface()).Error
					res = objValue.Elem().Interface()
				} else {
					if rt.Kind() >= reflect.Int && rt.Kind() <= reflect.Float64 || rt.Kind() == reflect.String {
						objValue := makeSlice(tk.TypeOfField, 0)
						err = tx.Pluck(fieldSel, objValue.Interface()).Error
						res = objValue.Elem().Interface()
					}
				}

			} else {
				objValue := makeSlice(reflect.TypeOf(ss.raw), 0)
				err = tx.Find(objValue.Interface()).Error
				res = objValue.Elem().Interface()
			}
		case reflect.Interface:
			tx = tx.Model(ss.raw)
			pi := new(interface{})
			*pi = tx.QueryExpr()
			return nil, e.f.Set(pi)
		}

		return res, err
	}

	var f1 = func(field *structs.Field, args ...interface{}) (*Structs, error) {
		tk := TypeKind(field)
		var strs *Structs
		if len(args) == 2 {
			m := args[1].(string)
			if len(m) > 0 {
				model := args[1].(string)
				strs = e.s.createModel(model)
			}
		} else {
			strs = tk.create()
		}
		params := strings.Split(args[0].(string), ",")
		return strs, strs.fillValue(e.s, params)
	}

	functions["qry"] = func(args ...interface{}) (interface{}, error) {
		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		q := newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		})

		if TypeKind(e.f).KindOfField == reflect.Interface {
			res, err := q.queryExpr()
			if err != nil {
				return nil, err
			}
			pi := new(interface{})
			*pi = res
			return nil, e.f.Set(pi)
		}
		return q.queryObj()
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

	var batchCreate = func(s *Structs, f *structs.Field, args ...interface{}) (interface{}, error) {
		value, err := batchfill(s, f, args...)
		if err != nil {
			return nil, err
		}
		slices := value.([]*Structs)
		for i := 0; i < len(slices); i++ {
			screate := slices[i]
			crud := newcrud(&config{
				strs:   screate,
				search: &SearchCondition{},
				db:     e.db.New(),
				ctx:    e.ctx,
			})
			if _, err := crud.createObj(); err != nil {
				return nil, err
			}
		}
		if len(slices) > 0 {
			objSlice := makeSlice(reflect.TypeOf(slices[0].raw), len(slices))
			objValue := objSlice.Elem()
			for i := 0; i < len(slices); i++ {
				screate := slices[i]
				objValue.Index(i).Set(reflect.ValueOf(screate.raw))
			}
			return objValue.Interface(), nil
		}
		return nil, nil
	}

	functions["create"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField == reflect.Slice {
			return batchCreate(e.s, e.f, args...)
		}

		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		return newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		}).createObj()
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
	var batchUpdate = func(s *Structs, f *structs.Field, args ...interface{}) (interface{}, error) {
		value, err := batchfill(s, f, args...)
		if err != nil {
			return nil, err
		}
		slices := value.([]*Structs)
		for i := 0; i < len(slices); i++ {
			screate := slices[i]
			crud := newcrud(&config{
				strs:   screate,
				search: &SearchCondition{},
				db:     e.db.New(),
				ctx:    e.ctx,
			})
			if _, err := crud.updateObj(); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	functions["update"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField == reflect.Slice {
			return batchUpdate(e.s, e.f, args...)
		}

		strs, err := f1(e.f, args...)
		if err != nil {
			return nil, err
		}

		return newcrud(&config{
			strs:   strs,
			search: &SearchCondition{},
			db:     e.db,
			ctx:    e.ctx,
		}).updateObj()
	}
	functions["vf"] = func(args ...interface{}) (interface{}, error) {
		if !args[0].(bool) {
			return nil, errors.New(args[1].(string))
		}
		return nil, nil
	}
}

var setParam = func(f *structs.Field, name string, params map[string]interface{}) {
	if f.Kind() == reflect.Interface {
		return
	}
	tk := TypeKind((f))
	if f.IsZero() {
		if reflect.TypeOf(f.Value()).Kind() == reflect.Ptr {
			params[name] = nil
		} else {
			if tk.KindOfField >= reflect.Int && tk.KindOfField <= reflect.Float32 {
				// 表达式比较只能返回float64
				a := float64(0)
				params[name] = a
			} else {
				params[name] = reflect.Zero(reflect.TypeOf(f.Value())).Interface()
			}
		}
	} else {
		if tk.KindOfField >= reflect.Int && tk.KindOfField <= reflect.Float32 {
			// 表达式比较只能返回float64
			v := DereferenceValue(reflect.ValueOf(f.Value()))
			a := float64(0)
			params[name] = v.Convert(reflect.TypeOf(a)).Interface()
		} else {
			params[name] = DereferenceValue(reflect.ValueOf(f.Value())).Interface()
		}
	}
}

var hasLetter = func(str string) bool {
	for _, r := range str {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

var sectionFunc = func(s *Structs, curf *structs.Field, sectionExp string, params map[string]interface{}) (string, error) {

	keys := []string{"create_if", "update_if", "set_if", "vf", "rd_create_if", "rd_update_if"}
	for _, k := range keys {
		if strings.HasPrefix(sectionExp, k) {
			//create_if(result==1 && name==hello;'user_id=id,user_name=name')
			//vf(len(split(this.name,','))==1;'error')
			//rd_update_if(company!=nil,'department=company.name';'name=billgates')
			a1 := strings.Index(sectionExp, "(")
			b1 := strings.LastIndex(sectionExp, "?")
			sectionExp = sectionExp[:b1] + string(",") + sectionExp[b1+1:]
			condition := sectionExp[a1+1 : b1]                   // result==1 && name==hello
			condition = strings.ReplaceAll(condition, ",", "$$") //等下要用，分割

			key := []string{"&&", "==", "<=", ">=", "||", ">", "<", "!="}
			for _, v := range key {
				condition = strings.ReplaceAll(condition, v, ",")
			}
			key = strings.Split(condition, ",")
			for _, v := range key {
				fieldName := strings.ReplaceAll(v, "$$", ",") //替换回来
				fieldName = trimSpace(fieldName)
				if len(fieldName) >= 2 && fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'' {
					continue // 'huang'
				}
				if len(fieldName) > 0 && hasLetter(fieldName) && fieldName != "nil" && params[fieldName] == nil {
					if a := strings.Index(fieldName, "(this."); a > -1 { // len(this.name) / len(split(this.name,','))
						b1 := strings.Index(fieldName[a+1:], ")")
						b2 := strings.Index(fieldName[a+1:], ",")
						if b1 != -1 && b2 != -1 {
							if b1 < b2 {
								fieldName = fieldName[a+1 : a+1+b1]
							} else {
								fieldName = fieldName[a+1 : a+1+b2]
							}
						} else if b1 != -1 {
							fieldName = fieldName[a+1 : a+1+b1]
						} else if b2 != -1 {
							fieldName = fieldName[a+1 : a+1+b2]
						} else {
							panic("")
						}
					}
					if strings.Contains(fieldName, ".") && !strings.Contains(fieldName, "'") {
						a := strings.Index(fieldName, ".")
						thisField := fieldName
						if thisField[:a] == "this" { // like this.name
							thisField = strings.Replace(fieldName, "this", curf.Name(), -1)
						}
						v, err := s.getValue(thisField)
						if err != nil {
							return sectionExp, err
						}
						str := strings.ReplaceAll(fieldName, ".", "_")
						str = strings.ReplaceAll(str, "[", "_")
						str = strings.ReplaceAll(str, "]", "_")
						sectionExp = strings.ReplaceAll(sectionExp, fieldName, str)
						params[str] = v

					} else if f, ok := s.FieldOk(ToCamel(fieldName)); ok {
						setParam(f, fieldName, params)
					}
				}
			}
		}
	}
	return sectionExp, nil
}

func (e *expr) eval(expString string) error {
	e.params["s"] = e.s.raw

	var res interface{}
	var err error

	strExpress := strings.ReplaceAll(expString, "||", "$$")
	sections := strings.Split(strExpress, "|")
	for _, section := range sections {
		section = strings.ReplaceAll(section, "$$", "||")
		section = trimSpace(section)
		setParam(e.f, "this", e.params)
		section, err = sectionFunc(e.s, e.f, section, e.params)
		if err != nil {
			return err
		}
		expression, err := govaluate.NewEvaluableExpressionWithFunctions(section, e.functions)
		if err != nil {
			return err
		}
		res, err = expression.Evaluate(e.params)
		if err != nil {
			return err
		}
		if res != nil {
			if err = e.s.SetFieldValue(e.f, res); err != nil {
				return err
			}
		}
	}
	return err
}

// Eval for test
func Eval(s *Structs, db *gorm.DB, f *structs.Field, exp string) error {
	expr := &expr{
		db:        db,
		s:         s,
		f:         f,
		functions: make(map[string]govaluate.ExpressionFunction),
		params:    make(map[string]interface{}),
	}
	expr.params["s"] = s.raw
	expr.params["nil"] = nil
	expr.init()

	return expr.eval(exp)
}
