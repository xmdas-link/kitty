package kitty

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"

	"github.com/Knetic/govaluate"
)

type modelCreateFunction func(name string) *Structs

type expr struct {
	db        *gorm.DB
	s         *Structs
	f         *structs.Field
	functions map[string]govaluate.ExpressionFunction
	params    map[string]interface{}
	ctx       Context
	createM   modelCreateFunction
}

func (e *expr) init() {
	e.params["nil"] = nil
	e.params["s"] = e.s.raw

	functions := e.functions
	for k, v := range exprFuncs {
		functions[k] = v
	}
	if e.createM == nil {
		e.createM = e.s.createModel
	}

	// 批量
	var batchfill = func(args ...interface{}) (interface{}, error) {
		count := float64(0)
		if len(args) > 0 {
			count = args[0].(float64)
		}
		tk := TypeKind(e.f)
		modelNameForCreate := strcase.ToSnake(tk.ModelName)
		slices := make([]*Structs, 0)
		if count > 0 {
			for i := 0; i < int(count); i++ {
				screate := tk.Create()
				slices = append(slices, screate)
			}
		} else {
			// 并不需要动态创建，则当前的字段是有值的。
			// 以当前的字段创建
			fvalue := e.f.Value()
			dslice := reflect.ValueOf(fvalue)
			for i := 0; i < dslice.Len(); i++ {
				screate := CreateModelStructs(dslice.Index(i).Interface())
				slices = append(slices, screate)
			}
		}

		for _, field := range e.s.Fields() {
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
		case "db":
			return e.db, nil
		case "loginid":
			return e.ctx.CurrentUID()
		default:
			return e.ctx.GetCtxInfo(s)
		}
	}

	functions["f"] = func(args ...interface{}) (interface{}, error) {
		strfield := args[0].(string)
		list := &fieldList{
			dst:       e.f,
			fieldStrs: e.s,
		}
		v, err := list.getValue(strfield)
		if err != nil {
			return nil, err
		}
		if str, ok := v.(string); ok {
			return trimConsts(str), nil
		}
		return v, nil
	}
	functions["set"] = func(args ...interface{}) (interface{}, error) {
		return args[0], nil
	}
	//创建单条记录
	functions["rd_create"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField == reflect.Slice {
			value, err := batchfill(args...)
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
		argv := args[0].(string)
		queryformat := []*fieldQryFormat{}
		v := strings.Split(argv, ",")
		for _, expression := range v {
			if strings.Contains(expression, "=") {
				vv := strings.Split(expression, "=")
				param := trimSpace(vv[1])
				res, err := e.s.getValue(param)
				if err != nil {
					return nil, err
				}
				if res == nil {
					continue
				}
				fname := strcase.ToSnake(trimSpace(vv[0]))
				queryformat = append(queryformat, &fieldQryFormat{
					bindfield: fname,
					model:     strcase.ToSnake(tk.ModelName),
					operator:  "= ?",
					value:     []interface{}{res},
				})
			}
		}
		qry := &simpleQuery{
			db:           e.db,
			ModelStructs: e.s,
			search:       &SearchCondition{},
			Result:       tk.Create(),
			qryParams:    queryformat,
		}
		return qry.create()
	}

	//更新单条记录  格式  update:xx=xx, where: xx=xx
	functions["rd_update"] = func(args ...interface{}) (interface{}, error) {
		updateCondition := args[0].(string)
		whereCondition := args[1].(string)

		tk := TypeKind(e.f)

		queryformat := []*fieldQryFormat{}
		vWhere := strings.Split(whereCondition, ",")
		for _, expression := range vWhere {
			operators := []string{" LIKE ", " IS NOT ", " IS ", " NOT IN ", " IN ", "<>", ">=", "<=", ">", "<", "="}
			for _, oper := range operators {
				if strings.Contains(expression, oper) {
					vv := strings.Split(expression, oper)
					param := trimSpace(vv[1])
					res, err := e.s.getValue(param)
					if err != nil {
						return nil, err
					}
					fname := strcase.ToSnake(trimSpace(vv[0]))
					queryformat = append(queryformat, &fieldQryFormat{
						bindfield:     fname,
						model:         strcase.ToSnake(tk.ModelName),
						withCondition: true,
						operator:      oper + " ?",
						value:         []interface{}{res},
					})
					break
				}
			}
		}
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
				queryformat = append(queryformat, &fieldQryFormat{
					bindfield: fname,
					model:     strcase.ToSnake(tk.ModelName),
					operator:  "= ?",
					value:     []interface{}{res},
				})
			}
		}

		var supdate *Structs
		if e.f.IsZero() {
			supdate = tk.Create()
			e.f.Set(supdate.raw)
		} else {
			supdate = CreateModelStructs(e.f.Value())
		}
		qry := &simpleQuery{
			db:           e.db,
			ModelStructs: e.s,
			search:       &SearchCondition{},
			Result:       supdate,
			qryParams:    queryformat,
		}
		return nil, qry.update()
	}

	// rds:  rds('key=value','user,a.field,field') 第二项不是必填项。
	// 当kitty字段不是gorm的时候，需声明第二项
	functions["rds"] = func(args ...interface{}) (interface{}, error) {
		tx := e.db
		tk := TypeKind(e.f)
		model := tk.ModelName
		modelDeclared := false
		fieldSel := ""
		modelAs := ""
		raw := ""
		var ss *Structs

		if len(args) >= 1 {
			if gormExpr, ok := args[0].(*interface{}); ok {
				raw = "raw"
				if len(args) >= 2 {
					model = args[1].(string)
					if len(model) > 0 {
						modelDeclared = true
						ss = e.createM(args[1].(string))
					}
				}
				if !modelDeclared {
					k := tk.TypeOfField
					if k.Kind() == reflect.Slice {
						k = DereferenceType(k.Elem())
					}
					if k.Kind() != reflect.Struct {
						return nil, fmt.Errorf("%s must a struct, or specify a model in the second param", e.f.Name())
					}
					ss = tk.Create()
				}
				tx = tx.Raw("?", *gormExpr)
			}
		}

		if len(args) == 3 {
			tx = tx.Order(args[2].(string))
		}

		if len(raw) == 0 { // 参数查询 product_id = product.id
			if len(args) >= 2 {
				fieldSel = args[1].(string)
				if strings.Contains(fieldSel, ".") {
					v := strings.Split(fieldSel, ".")
					model = v[0]
					fieldSel = v[1]
					modelDeclared = true
					if v = strings.Split(model, ","); len(v) == 2 {
						model = v[0]
						modelAs = v[1]
					}
				}
			}

			if modelDeclared {
				ss = e.createM(model)
				if len(modelAs) > 0 {
					tblname := tx.NewScope(ss.raw).TableName()
					tx = tx.Table(fmt.Sprintf("%s %s", tblname, modelAs))
				} else {
					tx = tx.Model(ss.raw)
				}
			} else {
				ss = tk.Create()
			}

			if len(args) > 0 {
				argv := args[0].(string)
				if v := strings.Split(argv, ","); len(v) > 0 {
					var fieldAs = func(field string) string {
						if len(modelAs) > 0 {
							return fmt.Sprintf("%s.%s", modelAs, field)
						}
						return field
					}
					for _, expression := range v {
						operators := []string{" LIKE ", " IS NOT ", " IS ", "<>", ">=", "<=", ">", "<", "=", " NOT IN ", " IN "}

						for _, oper := range operators {
							if strings.Contains(expression, oper) {
								vv := strings.Split(expression, oper)
								fname := strcase.ToSnake(trimSpace(vv[0]))
								param := trimSpace(vv[1])
								if len(param) >= 2 && param[0] == '[' && param[len(param)-1] == ']' {
									str := param[1 : len(param)-1]
									if len(str) == 0 {
										str = "''"
									}
									tx = tx.Where(fmt.Sprintf("%s %s %s", fieldAs(fname), oper, str))

								} else {
									res, err := e.s.getValue(param)
									if err != nil {
										return nil, err
									}
									tx = tx.Where(fmt.Sprintf("%s %s ?", fname, oper), res)
								}
								break
							}
						}
					}
				}
			}
		}

		// pages
		if pages, ok := e.s.FieldOk("Pages"); ok { // Pages kitty.Page `kitty:"page:List"`
			if k := pages.Tag("kitty"); strings.Contains(k, fmt.Sprintf("page:%s", e.f.Name())) {
				if !pages.IsZero() {
					pageInfo := pages.Value().(*Page)
					tx = tx.Offset(pageInfo.GetOffset()).Limit(pageInfo.Limit)
				} else {
					pageInfo := &Page{}
					if f, ok := e.s.FieldOk("Page"); ok {
						pageInfo.Page = f.Value().(uint32)
					}
					if f, ok := e.s.FieldOk("Limit"); ok {
						pageInfo.Limit = f.Value().(uint32)
					}

					var txPages *gorm.DB
					if len(raw) > 0 {
						gormExpr := args[0].(*interface{})
						txPages = e.db.Raw("SELECT COUNT(*) FROM (?) tmp", *gormExpr)
					} else {
						txPages = tx.Select("COUNT(*)")
						if !modelDeclared {
							txPages = txPages.Model(ss.raw)
						}
					}

					total := 0
					if err := txPages.Count(&total).Error; err != nil {
						return nil, err
					}
					pageInfo.CountPages(uint32(total))
					pages.Set(pageInfo)

					tx = tx.Offset(pageInfo.GetOffset()).Limit(pageInfo.Limit)
				}
			}

		}

		if len(fieldSel) > 0 {
			if !strings.Contains(fieldSel, ",") {
				tx = tx.Select(fmt.Sprintf("%s AS %s", fieldSel, strcase.ToSnake(e.f.Name())))
				fieldSel = strcase.ToSnake(e.f.Name())
			} else {
				tx = tx.Select(fieldSel)
			}
		}

		var err error
		var res interface{}

		switch tk.TypeOfField.Kind() {
		case reflect.Struct:
			if modelDeclared {
				result := tk.Create()
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
			if modelDeclared {
				rt := DereferenceType(tk.TypeOfField.Elem())
				if rt.Kind() == reflect.Struct {
					result := tk.Create()
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
			pi := new(interface{})
			*pi = tx.QueryExpr()
			return nil, e.f.Set(pi)
		default:
			if tk.TypeOfField.Kind() >= reflect.Int && tk.TypeOfField.Kind() <= reflect.Float64 || tk.TypeOfField.Kind() == reflect.String {
				objValue := makeSlice(tk.TypeOfField, 0)
				err = tx.Pluck(fieldSel, objValue.Interface()).Error
				if objValue.Elem().Len() > 0 {
					res = objValue.Elem().Index(0).Interface()
				}
			}
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
				strs = e.createM(model)
			}
		} else {
			strs = tk.Create()
		}
		params := strings.Split(args[0].(string), ",")
		return strs, strs.fillValue(e.s, params)
	}

	// qry kitty model
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

	// create kitty model
	functions["create"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField == reflect.Slice {
			value, err := batchfill(args...)
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

	// update kitty model
	functions["update"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField == reflect.Slice {
			value, err := batchfill(args...)
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

	functions["count"] = func(args ...interface{}) (interface{}, error) {
		if q, ok := args[0].(interface{}); ok {
			tx := e.db.Raw(fmt.Sprintf("SELECT COUNT(1) FROM (?) tmp"), q)
			tk := TypeKind(e.f)
			if tk.KindOfField == reflect.Interface {
				pi := new(interface{})
				*pi = tx.QueryExpr()
				return nil, e.f.Set(pi)
			}
			count := 0
			err := tx.Count(&count).Error
			return count, err
		}
		return nil, errors.New("kitty func count param error")
	}

	var If = func(name string, args ...interface{}) (interface{}, error) {
		if !args[0].(bool) {
			return nil, nil
		}
		fun := functions[name]
		return fun(args[1:]...)
	}
	functions["qry_if"] = func(args ...interface{}) (interface{}, error) {
		return If("qry", args...)
	}
	functions["create_if"] = func(args ...interface{}) (interface{}, error) {
		return If("create", args...)
	}
	functions["update_if"] = func(args ...interface{}) (interface{}, error) {
		return If("update", args...)
	}
	functions["set_if"] = func(args ...interface{}) (interface{}, error) {
		return If("set", args...)
	}
	functions["rd_create_if"] = func(args ...interface{}) (interface{}, error) {
		return If("rd_create", args...)
	}
	functions["rd_update_if"] = func(args ...interface{}) (interface{}, error) {
		return If("rd_update", args...)
	}
	functions["now"] = func(args ...interface{}) (interface{}, error) {
		tk := TypeKind(e.f)
		if tk.KindOfField >= reflect.Int && tk.KindOfField <= reflect.Float64 {
			return time.Now().UnixNano() / 1e6, nil
		}
		if tk.KindOfField == reflect.String {
			format := "2006-01-02 15:04:05"
			if len(args) == 1 {
				format = args[0].(string)
			}
			return time.Now().Format(format), nil
		}
		switch e.f.Value().(type) {
		case time.Time:
			return time.Now(), nil
		case *time.Time:
			t := time.Now()
			return &t, nil
		}
		return nil, fmt.Errorf("time now not support %s", e.f.Name())
	}

	functions["page"] = func(args ...interface{}) (interface{}, error) {
		// page(gorm.expr) // select count(*)
		Page := &Page{}
		var gormExpr interface{}

		if e, ok := args[0].(*interface{}); ok {
			gormExpr = *e
		} else if e, ok := args[0].(interface{}); ok {
			gormExpr = e
		} else {
			return nil, errors.New("page: param error")
		}

		e.db.Raw("?", gormExpr).Count(&Page.Total)
		if f, ok := e.s.FieldOk("Page"); ok {
			Page.Page = f.Value().(uint32)
		}
		if f, ok := e.s.FieldOk("Limit"); ok {
			Page.Limit = f.Value().(uint32)
		}
		Page.CountPages(Page.Total)
		return Page, nil
	}

}

func setParam(f *structs.Field, name string, params map[string]interface{}) {
	if f.Kind() == reflect.Interface {
		return
	}
	tk := TypeKind(f)
	if f.IsZero() {
		if reflect.TypeOf(f.Value()).Kind() == reflect.Ptr {
			params[name] = nil
		} else {
			if tk.KindOfField >= reflect.Int && tk.KindOfField <= reflect.Float32 {
				// 表达式比较只能返回float64
				params[name] = float64(0)
			} else {
				params[name] = reflect.Zero(reflect.TypeOf(f.Value())).Interface()
			}
		}
	} else {
		if tk.KindOfField >= reflect.Int && tk.KindOfField <= reflect.Float32 {
			// 表达式比较只能返回float64
			v := DereferenceValue(reflect.ValueOf(f.Value()))
			params[name] = v.Convert(reflect.TypeOf(float64(0))).Interface()
		} else {
			params[name] = DereferenceValue(reflect.ValueOf(f.Value())).Interface()
		}
	}
}

func hasLetter(str string) bool {
	for _, r := range str {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func sectionFunc(s *Structs, curf *structs.Field, sectionExp string, params map[string]interface{}) (string, error) {
	keys := []string{"create_if", "update_if", "set_if", "vf", "rd_create_if", "rd_update_if", "qry_if"}
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
				if len(fieldName) >= 2 && (fieldName[0] == '[' && fieldName[len(fieldName)-1] == ']' ||
					fieldName[0] == '\'' && fieldName[len(fieldName)-1] == '\'') {
					continue // [huang] [0] 'strings'
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
						if s1, ok := v.(string); ok {
							v = trimConsts(s1) // [billgates]
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
	expr.init()

	return expr.eval(exp)
}
