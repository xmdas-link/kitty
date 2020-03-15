package kitty

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"
	"github.com/modern-go/reflect2"
)

type QueryParam struct {
	model    interface{}
	field    string
	value    interface{}
	operator string
	link     string
	list     []*QueryParam
}

func (q *QueryParam) AND(model interface{}, field string, values ...interface{}) *QueryParam {
	qry := Param(model, field, values...)
	q.list = append(q.list, qry)
	return q
}

func (q *QueryParam) OR(model interface{}, field string, values ...interface{}) *QueryParam {
	qry := Param(model, field, values...)
	qry.link = " OR "
	q.list = append(q.list, qry)
	return q
}

func (q *QueryParam) And(field string, values ...interface{}) *QueryParam {
	qry := Param(q.model, field, values...)
	q.list = append(q.list, qry)
	return q
}

func (q *QueryParam) Or(field string, values ...interface{}) *QueryParam {
	qry := Param(q.model, field, values...)
	qry.link = " OR "
	q.list = append(q.list, qry)
	return q
}

func (q *QueryParam) format(db *gorm.DB) (query string, values []interface{}) {
	tblName := db.NewScope(q.model).TableName()
	qry := fmt.Sprintf("%s.%s %s", tblName, strcase.ToSnake(q.field), q.operator)

	query += qry
	values = append(values, q.value)

	for _, l := range q.list {
		tblName := db.NewScope(q.model).TableName()
		qry := fmt.Sprintf("%s.%s %s", tblName, strcase.ToSnake(l.field), l.operator)
		link := l.link
		if len(link) == 0 {
			link = " AND "
		}
		query += link + qry
		values = append(values, l.value)
	}
	if len(q.list) > 0 {
		query = fmt.Sprintf("(%s)", query)
	}

	return
}

func Param(model interface{}, field string, values ...interface{}) *QueryParam {
	qry := &QueryParam{
		model:    model,
		field:    field,
		operator: "= ?",
	}
	qry.value = values[0]
	if len(values) == 2 {
		qry.operator = values[1].(string) + " ?"
	} else {
		if reflect.ValueOf(qry.value).Kind() == reflect.Slice {
			qry.operator = "IN (?)"
		}
	}
	return qry
}

type OnJoin struct {
	key       string
	joinModel interface{}
	joinKey   string
	query     []*QueryParam
	values    []interface{}
}

func On(key, joinKey string, joinModel interface{}) *OnJoin {
	return &OnJoin{
		key:       key,
		joinModel: joinModel,
		joinKey:   joinKey,
	}
}

func (on *OnJoin) clone() *OnJoin {
	o := &OnJoin{
		key:       on.key,
		joinModel: on.joinModel,
		joinKey:   on.joinKey,
	}
	o.query = append(o.query, on.query...)
	o.values = append(o.values, on.values...)
	return o
}

func (on *OnJoin) And(q *QueryParam) *OnJoin {
	o := on.clone()
	o.query = append(o.query, q)
	return o
}

func (on *OnJoin) ON(joinName string, db *gorm.DB) (string, []interface{}) {
	s := " "
	if on.joinModel != nil {
		s += fmt.Sprintf("ON %s.%s = %s.%s",
			db.NewScope(on.joinModel).TableName(),
			strcase.ToSnake(on.key),
			joinName,
			strcase.ToSnake(on.joinKey))
	}
	if len(on.query) > 0 {
		s += " AND "
		var qry []string
		for _, q := range on.query {
			str, values := q.format(db)
			qry = append(qry, str)
			on.values = append(on.values, values...)
		}
		if len(qry) > 0 {
			s += strings.Join(qry, " AND ")
		}
	}
	return s, on.values
}

type join struct {
	way   string // left or right inner
	model interface{}
	on    *OnJoin
}

func (j *join) format(db *gorm.DB) (string, []interface{}) {
	s := j.way
	if len(s) > 0 {
		s += " "
	}
	// LEFT JOIN xxx ON xx.id = xxx.id AND
	name := db.NewScope(j.model).TableName()
	joinOn, values := j.on.ON(name, db)
	return s + "JOIN " + name + joinOn, values
}

type KK struct {
	Error      error
	db         *gorm.DB
	master     interface{} // for rds
	selField   string
	selModel   interface{}
	page       *Page
	joins      []*join
	queryParam []*QueryParam
	values     []interface{}
}

func New(db *gorm.DB) *KK {
	return &KK{
		db: db,
	}
}

func (kk *KK) clone() *KK {
	cl := &KK{
		Error:    kk.Error,
		master:   kk.master,
		selField: kk.selField,
		page:     kk.page,
		db:       kk.db,
		selModel: kk.selModel,
	}
	cl.joins = append(cl.joins, kk.joins...)
	cl.queryParam = append(cl.queryParam, kk.queryParam...)
	cl.values = append(cl.values, kk.values...)
	return cl
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func (kk *KK) Set(dst, given interface{}) *KK {
	cl := kk.clone()
	if cl.Error != nil {
		return cl
	}

	if err, ok := given.(error); ok {
		cl.Error = err
		return cl
	}

	dstValue := indirect(reflect.ValueOf(dst))

	if !dstValue.CanSet() {
		cl.Error = errors.New("KK.Set dst can not set")
		return cl
	}

	givenValue := reflect.ValueOf(given)

	var sameKind = func() bool {
		if givenValue.Kind() != dstValue.Kind() {
			return false
		}
		// *interface{}
		if _, ok := dst.(*interface{}); ok && givenValue.Kind() == reflect.Ptr && givenValue.Elem().Kind() == reflect.Interface {
			return true
		}

		if dstValue.Kind() == reflect.Ptr || dstValue.Kind() == reflect.Slice {
			v1 := DereferenceValue(givenValue).Kind()
			if v1 != DereferenceType(reflect.TypeOf(dst)).Kind() {
				return false
			}
		}
		return true
	}

	var set = func(dst, value reflect.Value) error {
		if dst.Kind() == value.Kind() {
			dst.Set(value)
			return nil
		}
		return fmt.Errorf("kk.Set Not Support kind. %s want: %s", value.Kind(), dst.Kind())
	}

	if sameKind() {
		cl.Error = set(dstValue, givenValue)
		return cl
	}

	dstType := reflect.TypeOf(dst)
	FT := DereferenceType(dstType)
	FK := FT.Kind()
	givenValue = DereferenceValue(givenValue)

	var f1 = func(rv reflect.Value) *KK {
		if dstType.Kind() != reflect.Ptr {
			cl.Error = set(dstValue, reflect.ValueOf(rv.Interface()))
		} else {
			cl.Error = set(dstValue, reflect.ValueOf(ptr(rv).Interface()))
		}
		return cl
	}

	switch dst.(type) {
	case time.Time:
		if givenValue.Kind() == reflect.String {
			if stamp, err := time.ParseInLocation("2006-01-02 15:04:05", given.(string), time.Local); err == nil {
				return f1(reflect.ValueOf(stamp))
			}
		}
		if givenValue.Kind() >= reflect.Int && givenValue.Kind() <= reflect.Float64 {
			str := fmt.Sprintf("%v", given)
			x, _ := strconv.ParseInt(str, 10, 64)
			stamp := time.Unix(x, 0)
			return f1(reflect.ValueOf(stamp))
		}
		cl.Error = fmt.Errorf("%v 时间格式错误", givenValue)
		return cl
	case *time.Time:
		if givenValue.Kind() == reflect.String {
			if len(dst.(string)) == 0 {
				return nil
			}
			if stamp, err := time.ParseInLocation("2006-01-02 15:04:05", given.(string), time.Local); err == nil {
				return f1(reflect.ValueOf(stamp))
			}
		}
		if givenValue.Kind() >= reflect.Int && givenValue.Kind() <= reflect.Float64 {
			str := fmt.Sprintf("%v", given)
			x, _ := strconv.ParseInt(str, 10, 64)
			stamp := time.Unix(x, 0)
			return f1(reflect.ValueOf(stamp))
		}
		cl.Error = fmt.Errorf("%v 时间格式错误", givenValue)
		return cl
	case bool, *bool:
		zero := reflect.Zero(givenValue.Type()).Interface()
		return f1(reflect.ValueOf(!reflect.DeepEqual(givenValue.Interface(), zero)))
	}

	var x interface{}

	if givenValue.Kind() == reflect.String {
		switch FK {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			x, _ = strconv.ParseInt(givenValue.Interface().(string), 10, 64)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			x, _ = strconv.ParseUint(givenValue.Interface().(string), 10, 64)
		case reflect.Float32, reflect.Float64:
			x, _ = strconv.ParseFloat(givenValue.Interface().(string), 64)
		}
	} else if FK >= reflect.Int && FK <= reflect.Float64 && givenValue.Kind() >= reflect.Int && givenValue.Kind() <= reflect.Float64 { // 同为整型
		x = givenValue.Interface()
	} else if FK == reflect.String && givenValue.Kind() >= reflect.Int && givenValue.Kind() <= reflect.Float64 {
		str := fmt.Sprintf("%v", givenValue)
		x = reflect.ValueOf(str).Interface()
	}

	if x != nil {
		v1 := reflect.ValueOf(x).Convert(FT)
		return f1(v1)
	}

	if FK == reflect.Struct || FK == reflect.Slice || FK == reflect.Map {
		cl.Error = set(dstValue, reflect.ValueOf(given))
	}

	return cl
}

// rds的src不是gorm模型，并且rds的查询参数不是gorm表达式
func (kk *KK) Model(value interface{}) *KK {
	cl := kk.clone()
	cl.master = value
	return cl
}

// count(*) sum(xxx) average(xxx) date_format field1,field2
func (kk *KK) Select(value interface{}) *KK {
	cl := kk.clone()
	if s, ok := value.(string); ok {
		cl.selField = s
	} else {
		cl.selModel = value
	}

	return cl
}

func (kk *KK) Order(value interface{}, reorder ...bool) *KK {
	cl := kk.clone()
	cl.db = cl.db.Order(value, reorder...)
	return cl
}

func (kk *KK) Where(q *QueryParam) *KK {
	cl := kk.clone()
	cl.queryParam = append(cl.queryParam, q)
	return cl
}

func (kk *KK) And(q *QueryParam) *KK {
	cl := kk.clone()
	q.link = " AND "
	cl.queryParam = append(cl.queryParam, q)
	return cl
}

func (kk *KK) Or(q *QueryParam) *KK {
	cl := kk.clone()
	q.link = " OR "
	cl.queryParam = append(cl.queryParam, q)
	return cl
}

func (kk *KK) Page(page *Page) *KK {
	cl := kk.clone()
	cl.page = page
	return cl
}

func type2(v interface{}) reflect2.Type {
	typ := reflect2.TypeOf(v)
	for typ.Kind() == reflect.Ptr {
		typ = typ.(*reflect2.UnsafePtrType).Elem()
	}
	if typ.Kind() == reflect.Struct {
		return typ
	} else if typ.Kind() == reflect.Slice {
		sliceType := typ.(*reflect2.UnsafeSliceType)
		elemType := sliceType.Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.(*reflect2.UnsafePtrType).Elem()
		}
		if elemType.Kind() == reflect.Struct {
			return elemType
		}
	}
	return typ
}

//kk.Model().Select().Rds(&field,"id=?",user.id)
//kk.Select("Count(*)").Rds(&field)
//kk.Select("Sum(money)").Rds(&field)
//kk.Page().Rds(&field,kk.GormQueryExpr())
func (kk *KK) Rds(args ...interface{}) *KK {
	cl := kk.clone()
	if cl.Error != nil {
		return cl
	}

	var (
		dst                = args[0]
		dstValue           = DereferenceValue(reflect.ValueOf(dst))
		dstType            = DereferenceType(reflect.TypeOf(dst))
		tx                 = cl.db
		isGormQueryExpr    = false
		searchOneFieldName = "KittyTempField"
	)

	if !dstValue.CanSet() {
		//		cl.Error = errors.New("KK.Rds dst can not set")
		//		return cl
		//if dstType.Kind() == reflect.Struct {
		//		reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(type2(dst).New()))
		//	}
	}

	if len(args) == 2 {
		if expr, ok := args[1].(*interface{}); ok {
			isGormQueryExpr = true
			tx = tx.Raw("?", *expr)
			if cl.master == nil {
				cl.master = dst // gorm queryexpr not need declared.
			}
		} else {
			tx = tx.Where(args[1])
		}
	} else if len(args) > 2 {
		tx = tx.Where(args[1], args[2:]...)
	}

	if len(cl.queryParam) > 0 {
		query := ""
		var values []interface{}
		for _, q := range cl.queryParam {
			str, vs := q.format(cl.db)
			link := q.link
			if len(link) == 0 && len(query) > 0 {
				link = " AND "
			}
			query += link + str
			values = append(values, vs...)
		}
		if len(query) > 0 {
			tx = tx.Where(query, values...)
		}
	}

	// 非gorm表达式
	if !isGormQueryExpr {
		if cl.master != nil {
			tx = tx.Model(cl.master)
		} else {
			k := dstType
			if k.Kind() == reflect.Slice {
				k = DereferenceType(k.Elem())
			}
			if k.Kind() != reflect.Struct {
				cl.Error = fmt.Errorf("kk.rds: %s must a struct", dstType.Name())
				return cl
			}

			tx = tx.Model(type2(dst).New())
		}
	}

	// join.
	if len(cl.joins) > 0 {
		for _, join := range cl.joins {
			action, values := join.format(tx)
			if len(values) > 0 {
				tx = tx.Joins(action, values...)
			} else {
				tx = tx.Joins(action)
			}
		}
	}

	// 分页
	if cl.page != nil && cl.page.Page > 0 && cl.page.Limit > 0 {
		var txPages *gorm.DB
		if isGormQueryExpr {
			gormExpr := args[1].(*interface{})
			txPages = tx.New().Raw("SELECT COUNT(*) FROM (?) tmp", *gormExpr)
		} else {
			txPages = tx.Select("COUNT(*)")
		}

		total := 0
		if err := txPages.Count(&total).Error; err != nil {
			cl.Error = err
			return cl
		}
		cl.page.CountPages(uint32(total))
		tx = tx.Offset(cl.page.GetOffset()).Limit(cl.page.Limit)
	}

	if !isGormQueryExpr {
		var selectModelFields []string
		if cl.selModel != nil {
			curTableName := ""
			typ := reflect2.TypeOf(cl.selModel)
			for typ.Kind() == reflect.Ptr {
				typ = typ.(*reflect2.UnsafePtrType).Elem()
			}
			structType := typ.(*reflect2.UnsafeStructType)
			for i := 0; i < structType.NumField(); i++ {
				field := structType.Field(i)
				if field.Type().Kind() == reflect.Ptr && strings.HasPrefix(field.Name(), "bind") {
					ptrType := field.Type().(*reflect2.UnsafePtrType)
					if ptrType.Elem().Kind() == reflect.Struct {
						curTableName = tx.NewScope(ptrType.Elem().New()).TableName()
					}
				} else {
					fieldName := strcase.ToSnake(field.Name())
					name := fieldName
					if tag := field.Tag().Get("bind"); len(tag) > 0 {
						name = strcase.ToSnake(tag)
						name = fmt.Sprintf("%s.%s as %s", curTableName, name, fieldName)
					} else {
						name = fmt.Sprintf("%s.%s", curTableName, name)
					}
					selectModelFields = append(selectModelFields, name)
				}
			}
		}

		if len(cl.selField) > 0 {
			if strings.Contains(cl.selField, ",") || strings.Contains(cl.selField, " as ") || strings.Contains(cl.selField, " AS ") {
				selectModelFields = append(selectModelFields, cl.selField)
			} else {
				selectModelFields = append(selectModelFields, fmt.Sprintf("%s AS %s", cl.selField, strcase.ToSnake(searchOneFieldName)))
			}
		}
		if len(selectModelFields) > 0 {
			tx = tx.Select(strings.Join(selectModelFields, ", "))
		}
	}

	switch dstType.Kind() {
	case reflect.Struct:
		v := type2(dst).New()
		if cl.master != nil {
			tx = tx.Scan(v)
		} else {
			tx = tx.First(v)
		}
		if !tx.RecordNotFound() {
			t := reflect.TypeOf(dst)
			if t.Elem().Kind() == reflect.Ptr {
				reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(v))
			} else {
				reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(v).Elem())
			}
		}
	case reflect.Slice:
		if cl.master != nil {
			rt := DereferenceType(dstType.Elem())
			if rt.Kind() == reflect.Struct {
				cl.Error = tx.Scan(dst).Error
			} else {
				if rt.Kind() >= reflect.Int && rt.Kind() <= reflect.Float64 || rt.Kind() == reflect.String {
					sType := reflect.StructOf([]reflect.StructField{
						{
							Name: searchOneFieldName,
							Type: dstType.Elem(),
						},
					})
					sv := reflect.New(sType)
					objValue := makeSlice(reflect.TypeOf(sv.Interface()), 0)
					cl.Error = tx.Scan(objValue.Interface()).Error

					count := objValue.Elem().Len()
					v1 := makeSlice(dstType, count)
					for i := 0; i < count; i++ {
						sv := CreateModelStructs(objValue.Elem().Index(i).Interface())
						v1.Elem().Index(i).Set(reflect.ValueOf(sv.Field(searchOneFieldName).Value()))
					}
					return cl.Set(dst, v1.Elem().Interface())
				}
				cl.Error = fmt.Errorf("kk.rds: not support kind %s", rt.Kind())
			}
		} else {
			cl.Error = tx.Find(dst).Error
		}
	default:
		rt := dstType
		if rt.Kind() >= reflect.Int && rt.Kind() <= reflect.Float64 || rt.Kind() == reflect.String {
			sType := reflect.StructOf([]reflect.StructField{
				{
					Name: searchOneFieldName,
					Type: dstType,
				},
			})
			sv := reflect.New(sType)
			svi := sv.Interface()
			cl.Error = tx.Scan(svi).Error
			ss := CreateModelStructs(svi)
			return cl.Set(dst, ss.Field(searchOneFieldName).Value())
		}
		cl.Error = fmt.Errorf("kk.rds: not support kind %s", rt.Kind())
	}

	return cl
}

func (kk *KK) Verify(success bool, err string) *KK {
	cl := kk.clone()

	if !success {
		cl.Error = errors.New(err)
		return cl
	}
	if cl.Error != nil {
		return cl
	}

	return cl
}

func (kk *KK) GormQueryExpr(expr interface{}) interface{} {
	pi := new(interface{})
	*pi = expr
	return pi
}

func (kk *KK) LeftJoin(model interface{}, on *OnJoin) *KK {
	return kk.join("LEFT", model, on)
}

func (kk *KK) RightJoin(model interface{}, on *OnJoin) *KK {
	return kk.join("RIGHT", model, on)
}

func (kk *KK) Join(model interface{}, on *OnJoin) *KK {
	return kk.join("", model, on)
}

func (kk *KK) join(way string, model interface{}, on *OnJoin) *KK {
	cl := kk.clone()
	if cl.Error != nil {
		return cl
	}

	if on.joinModel == nil {
		if cl.master == nil {
			cl.Error = errors.New("kk.join: master model or join model required")
			return cl
		}
		on.joinModel = cl.master
	}
	cl.joins = append(cl.joins, &join{
		way:   way,
		model: model,
		on:    on,
	})
	return cl
}
