package kitty

import (
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/modern-go/reflect2"

	"github.com/fatih/structs"
)

// Structs .
type Structs struct {
	*structs.Struct
	raw interface{}
}

// FieldTypeAndKind 字段类型，模型名称
type FieldTypeAndKind struct {
	ModelName   string       //模型名称
	KindOfField reflect.Kind //类型  struct
	TypeOfField reflect.Type //类型
	t2          reflect2.Type
}

// Create ..
func (f FieldTypeAndKind) Create() *Structs {
	if f.t2 != nil {
		return CreateModelStructs(f.t2.New())
	}
	log.Panicf("model: %s must be declared", f.ModelName)
	return nil
}

// StructFieldInfo 结构体信息
type StructFieldInfo struct {
	TypeKind              FieldTypeAndKind
	ForeignKey            string //外键
	AssociationForeignkey string //关联外键
	Relationship          string //belongs_to has_one has_many
}

// fieldQryFormat 参数字段查询格式化
// IN / LIKE / Between.And / = / >= <=
type fieldQryFormat struct {
	model         string        // 模型名称
	fname         string        // structs名称
	bindfield     string        // 数据库字段名称
	operator      string        // 比较方式
	value         []interface{} // 具体的值
	withCondition bool          // update where condition
	order         bool
	format        string // like format:sum($)
}

func (f *fieldQryFormat) nullExpr() string {
	if len(f.value) == 1 {
		if str, ok := f.value[0].(string); ok && str == "[NULL]" {
			op := strings.ReplaceAll(f.operator, "(?)", "")
			op = strings.ReplaceAll(op, "?", "")
			return fmt.Sprintf("%s %s %s", f.bindfield, trimSpace(op), trimConsts(str))
		}
	}
	return ""
}

func (f *fieldQryFormat) gormExpr() interface{} {
	if len(f.value) == 1 {
		if g, ok := f.value[0].(*interface{}); ok {
			return *g
		}
	}
	return nil
}

func (f *fieldQryFormat) whereExpr() string {
	return fmt.Sprintf("%s %s", f.bindfield, f.operator)
}

func (f *fieldQryFormat) orderExpr() string {
	v := DereferenceValue(reflect.ValueOf(f.value[0])).Interface().(int)
	if v > 0 {
		return fmt.Sprintf("%s ASC", f.bindfield)
	}
	return fmt.Sprintf("%s DESC", f.bindfield)
}

func (s *Structs) createModel(name string) *Structs {
	modelname := strcase.ToSnake(name)
	var createStruct = func(field reflect2.Type) *Structs {
		nativeType := DereferenceType(field.Type1())
		if strcase.ToSnake(nativeType.Name()) == modelname {
			return CreateModelStructs(field.New())
		}
		return nil
	}

	typ := reflect2.TypeOf(s.raw)
	structType := (typ.(*reflect2.UnsafePtrType)).Elem().(*reflect2.UnsafeStructType)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.Type().Kind() == reflect.Struct {
			if v := createStruct(field.Type()); v != nil {
				return v
			}
		} else if field.Type().Kind() == reflect.Ptr {
			ptrType := field.Type().(*reflect2.UnsafePtrType)
			if ptrType.Elem().Kind() == reflect.Struct {
				if v := createStruct(ptrType.Elem()); v != nil {
					return v
				}
			}
		} else if field.Type().Kind() == reflect.Slice {
			sliceType := field.Type().(*reflect2.UnsafeSliceType)
			elemType := sliceType.Elem()
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.(*reflect2.UnsafePtrType).Elem()
			}
			if elemType.Kind() == reflect.Struct {
				if v := createStruct(elemType); v != nil {
					return v
				}
			}
		}
	}

	log.Panicf("model %s must be declared", name)
	return nil
}

// CreateModelStructs ...
func CreateModelStructs(v interface{}) *Structs {
	s := &Structs{structs.New(v), v}
	return s
}

// New a obj
func (s *Structs) New() *Structs {
	t := reflect2.TypeOf(s.raw).(*reflect2.UnsafePtrType).Elem()
	return CreateModelStructs(t.New())
}

// Raw return obj
func (s *Structs) Raw() interface{} {
	return s.raw
}

// CallMethod .
func (s *Structs) CallMethod(name string, values ...reflect.Value) []reflect.Value {
	getValue := reflect.ValueOf(s.raw)
	methodValue := getValue.MethodByName(name)
	if !methodValue.IsValid() {
		return []reflect.Value{}
	}
	argv := make([]reflect.Value, methodValue.Type().NumIn())
	for i, v := range values {
		argv[i] = v
	}
	return methodValue.Call(argv)
}

// SetFieldValue ...
func (s *Structs) SetFieldValue(f *structs.Field, value interface{}) error {

	var sameKind = func() bool {
		rv := reflect.ValueOf(value)
		if rv.Kind() != f.Kind() {
			return false
		}
		// *interface{}
		switch f.Value().(type) {
		case *interface{}:
			if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Interface {
				return true
			}
		}

		if f.Kind() == reflect.Ptr || f.Kind() == reflect.Slice {
			v1 := DereferenceValue(rv).Kind()
			if v1 != TypeKind(f).KindOfField {
				return false
			}
		}
		return true
	}
	if sameKind() {
		return f.Set(value)
	}

	rv := DereferenceValue(reflect.ValueOf(value))
	VK := rv.Kind()

	RealType := reflect.TypeOf(f.Value())
	FT := DereferenceType(RealType)
	FK := FT.Kind()

	var f1 = func(rv reflect.Value) error {
		var err error
		if RealType.Kind() != reflect.Ptr {
			err = f.Set(rv.Interface())
		} else {
			err = f.Set(ptr(rv).Interface())
		}
		if err != nil {
			return fmt.Errorf("%s: %s", f.Name(), err.Error())
		}
		return nil
	}
	// 同一类型
	if VK == FK {
		return f1(rv)
	}

	switch f.Value().(type) {
	case time.Time:
		if VK == reflect.String {
			if stamp, err := time.ParseInLocation("2006-01-02 15:04:05", rv.Interface().(string), time.Local); err == nil {
				return f1(reflect.ValueOf(stamp))
			}
		}
		if VK >= reflect.Int && VK <= reflect.Float64 {
			str := fmt.Sprintf("%v", rv)
			x, _ := strconv.ParseInt(str, 10, 64)
			stamp := time.Unix(x, 0)
			return f1(reflect.ValueOf(stamp))
		}
		return fmt.Errorf("%s: %v 时间格式错误", f.Name(), rv)
	case *time.Time:
		if VK == reflect.String {
			if len(rv.Interface().(string)) == 0 {
				return nil
			}
			if stamp, err := time.ParseInLocation("2006-01-02 15:04:05", rv.Interface().(string), time.Local); err == nil {
				return f1(reflect.ValueOf(stamp))
			}
		}
		if VK >= reflect.Int && VK <= reflect.Float64 {
			str := fmt.Sprintf("%v", rv)
			x, _ := strconv.ParseInt(str, 10, 64)
			stamp := time.Unix(x, 0)
			return f1(reflect.ValueOf(stamp))
		}
		return fmt.Errorf("%s: %v 时间格式错误", f.Name(), rv)
	case bool, *bool:
		zero := reflect.Zero(rv.Type()).Interface()
		return f1(reflect.ValueOf(!reflect.DeepEqual(rv.Interface(), zero)))
	}

	var x interface{}

	if VK == reflect.String {
		switch FK {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			x, _ = strconv.ParseInt(rv.Interface().(string), 10, 64)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			x, _ = strconv.ParseUint(rv.Interface().(string), 10, 64)
		case reflect.Float32, reflect.Float64:
			x, _ = strconv.ParseFloat(rv.Interface().(string), 64)
		}
	} else if FK >= reflect.Int && FK <= reflect.Float64 && VK >= reflect.Int && VK <= reflect.Float64 { // 同为整型
		x = rv.Interface()
	} else if FK == reflect.String && VK >= reflect.Int && VK <= reflect.Float64 {
		str := fmt.Sprintf("%v", rv)
		x = reflect.ValueOf(str).Interface()
	}

	if x != nil {
		v1 := reflect.ValueOf(x).Convert(FT)
		return f1(v1)
	}

	if FK == reflect.Struct || FK == reflect.Slice || FK == reflect.Map {
		if err := f.Set(value); err != nil {
			return fmt.Errorf("%s: %s", f.Name(), err.Error())
		}
		return nil
	}

	return fmt.Errorf("%s: Not Support kind. %s want: %s", f.Name(), VK, FK)
}

// SetValue ...key 参数为蛇形 入 name_of_who
func (s *Structs) SetValue(values map[string]interface{}) error {
	for k := range values {
		if _, ok := s.FieldOk(ToCamel(k)); !ok {
			return fmt.Errorf("field error %v", k)
		}
	}
	for _, f := range s.Fields() {
		name := strcase.ToSnake(f.Name())
		if v, ok := values[name]; ok {
			if err := s.SetFieldValue(f, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// fillValue 从一个结构体赋值
// params 可能是字段，可能是value
// params like: name=username id=1 id=product.id id=product.data.id
func (s *Structs) fillValue(src *Structs, params []string) error {
	for _, param := range params {
		p := strings.Split(param, "=")
		if len(p) != 2 {
			log.Panicf("%s fillvalue %s", s.Name(), strings.Join(params, ","))
		}
		field := s.Field(ToCamel(trimSpace(p[0])))
		value, err := src.getValue(trimSpace(p[1]))
		if err != nil {
			return err
		}
		//	if str, ok := value.(string); ok {
		//		str = trimConsts(str)
		//	}
		if value != nil {
			if err := s.SetFieldValue(field, value); err != nil {
				return err
			}
		}
	}
	return nil
}

type fieldList struct {
	dst       *structs.Field
	field     *structs.Field
	fieldStrs *Structs
	isSlice   bool
	parent    *fieldList
}

func (list *fieldList) getValue(param string) (interface{}, error) {
	if len(param) >= 2 && param[0] == '[' && param[len(param)-1] == ']' {
		return param, nil
	}
	if strings.Contains(param, ".") {
		vv := strings.Split(param, ".")
		fieldName := vv[0]
		sliceIdx := ""
		if i := strings.Index(fieldName, "["); i > 0 {
			b := strings.Index(fieldName, "]")
			sliceIdx = fieldName[i+1 : b]
			fieldName = fieldName[:i]
		}
		field, ok := list.fieldStrs.FieldOk(ToCamel(fieldName))
		if !ok {
			return nil, fmt.Errorf("field %s not exist", fieldName)
		}
		if field.IsZero() {
			return nil, nil
		}
		fieldvalue := field.Value()
		list.field = field
		tk := TypeKind(field)
		switch tk.KindOfField {
		case reflect.Slice: // field[0].Name
			list.isSlice = true
			if len(sliceIdx) == 0 {
				return nil, fmt.Errorf("field %s slice index error", field.Name())
			}
			slicevalue := DereferenceValue(reflect.ValueOf(fieldvalue))
			if sliceIdx != "*" {
				idx, _ := strconv.ParseInt(sliceIdx, 10, 64)
				if slicevalue.Len() < int(idx) {
					return nil, fmt.Errorf("slice idx overflow %s", field.Name())
				}
				fieldvalue = slicevalue.Index(int(idx)).Interface()
			} else if slicevalue.Len() > 0 {
				fieldvalue = slicevalue.Index(0).Interface()
			}
		default:
			if len(sliceIdx) > 0 {
				return nil, fmt.Errorf("field %s is not slice", field.Name())
			}
		}
		ss := CreateModelStructs(fieldvalue)
		fieldNewList := &fieldList{
			dst:       list.dst,
			fieldStrs: ss,
			parent:    list,
		}
		p := strings.Join(vv[1:], ".")
		return fieldNewList.getValue(p)
	}
	fieldName := param
	sliceIdx := ""
	if i := strings.Index(fieldName, "["); i > 0 {
		b := strings.Index(fieldName, "]")
		sliceIdx = fieldName[i+1 : b]
		fieldName = fieldName[:i]
	}
	if f, ok := list.fieldStrs.FieldOk(ToCamel(fieldName)); ok {
		if f.IsZero() {
			return nil, nil
		}
		tk := TypeKind(f)
		if tk.KindOfField == reflect.Interface {
			return reflect.ValueOf(f.Value()).Elem().Interface(), nil
		}
		if tk.KindOfField == reflect.Slice && list.dst != nil {
			slicevalue := DereferenceValue(reflect.ValueOf(f.Value()))
			if slicevalue.Len() == 0 {
				return nil, nil
			}
			dstKind := TypeKind(list.dst)
			if dstKind.KindOfField == reflect.Struct {
				idx, _ := strconv.ParseInt(sliceIdx, 10, 64)
				if slicevalue.Len() < int(idx) {
					return nil, fmt.Errorf("slice idx overflow %s", f.Name())
				}
				fieldvalue := slicevalue.Index(int(idx)).Interface()
				if dstKind.ModelName != tk.ModelName {
					src := CreateModelStructs(fieldvalue)
					ss := dstKind.Create()
					ss.Copy(src)
					return ss.raw, nil
				}
				return fieldvalue, nil
			} else if dstKind.KindOfField == reflect.Slice {
				if dstKind.ModelName == tk.ModelName {
					return slicevalue.Interface(), nil
				}
				//同为切片，但结构体不一样。复制。
				ty := DereferenceType(dstKind.TypeOfField.Elem())
				if ty.Kind() == reflect.Struct {
					objValue := makeSlice(dstKind.TypeOfField, slicevalue.Len())
					for i := 0; i < slicevalue.Len(); i++ {
						fieldvalue := slicevalue.Index(i).Interface()
						src := CreateModelStructs(fieldvalue)
						ss := dstKind.Create()
						ss.Copy(src)
						objValue.Elem().Index(i).Set(reflect.ValueOf(ss.raw))
					}
					return objValue.Elem().Interface(), nil
				}
			}
			return nil, fmt.Errorf("model does not match %s", f.Name())
		}

		// Fields[*].Name -> []string 取所有切片的字段
		if list.dst != nil && list.parent != nil && (tk.KindOfField >= reflect.Bool && tk.KindOfField <= reflect.Float64 ||
			tk.KindOfField == reflect.String) && TypeKind(list.dst).KindOfField == reflect.Slice && list.parent.isSlice {
			slicevalue := DereferenceValue(reflect.ValueOf(list.parent.field.Value()))
			if slicevalue.Len() == 0 {
				return nil, nil
			}
			fieldvalue := slicevalue.Index(0).Interface()
			ss := CreateModelStructs(fieldvalue)
			field := ss.Field(f.Name())
			objValue := makeSlice(TypeKind(field).TypeOfField, slicevalue.Len())
			for i := 0; i < slicevalue.Len(); i++ {
				fieldvalue = slicevalue.Index(i).Interface()
				ss := CreateModelStructs(fieldvalue)
				field := ss.Field(f.Name())
				objValue.Elem().Index(i).Set(reflect.ValueOf(field.Value()))
			}
			return objValue.Elem().Interface(), nil
		}

		if tk.KindOfField >= reflect.Int && tk.KindOfField <= reflect.Float32 {
			// 表达式比较只能返回float64
			v := DereferenceValue(reflect.ValueOf(f.Value()))
			return v.Convert(reflect.TypeOf(float64(0))).Interface(), nil
		}
		return DereferenceValue(reflect.ValueOf(f.Value())).Interface(), nil
	}
	return param, nil
}

// param可能是字段，也可能普通字符串. 如果非字段，则返回该param
// param可能包含及联，则遇到slice的时候，默认读取第一个。
// params like: name=username id=1 id=product.id id=product.data.id
// name=function('abcd')
func (s *Structs) getValue(param string) (interface{}, error) {
	list := &fieldList{
		fieldStrs: s,
	}
	return list.getValue(param)
}

// GetRelationsWithModel fieldname (elem) must struct -> email = user
func (s *Structs) GetRelationsWithModel(fieldname string, modelName string) (fi StructFieldInfo, err error) {

	if field, ok := s.FieldOk(fieldname); ok {

		tag := field.Tag("gorm")
		if len(tag) > 0 {
			keys := strings.Split(tag, ";")
			for _, key := range keys {
				if strings.Contains(key, "association_foreignkey") {
					fi.AssociationForeignkey = strings.Split(key, ":")[1]
				} else if strings.Contains(key, "foreignkey") {
					fi.ForeignKey = strings.Split(key, ":")[1]
				}
			}
		}

		if len(fi.AssociationForeignkey) == 0 {
			fi.AssociationForeignkey = "ID"
		}

		testNewForeignKey := false
		if len(fi.ForeignKey) == 0 {
			fi.ForeignKey = modelName + "ID" // UserID
			testNewForeignKey = true
		}

		if testNewForeignKey {
			ss := s.createModel(modelName) //NewModelStruct(modelName)
			if _, ok := ss.FieldOk(fi.AssociationForeignkey); ok {
				fi.Relationship = "has_one"
			} else {
				fi.Relationship = "nothing"
			}
		}
		return fi, nil
	}
	return fi, fmt.Errorf("invalid field %s", fieldname)
}

// ParseFormValues form_value -> struct
func (s *Structs) ParseFormValues(values url.Values) error {
	for _, field := range s.Fields() {
		k := field.Tag("kitty")
		if len(k) > 0 && strings.Contains(k, "param") && !strings.Contains(k, "-;param") {
			formfield := strcase.ToSnake(field.Name())
			if formvalue, ok := values[formfield]; ok {
				fk := TypeKind(field)
				if fk.KindOfField == reflect.Slice {
					if err := s.SetFieldValue(field, formvalue[:]); err != nil {
						return err
					}
				} else if err := s.SetFieldValue(field, formvalue[0]); err != nil {
					return err
				}
			}
		}

	}
	return nil
}

func isNil(field *structs.Field) bool {
	switch field.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Interface:
		if reflect.ValueOf(field.Value()).IsNil() {
			return true
		}
	}
	return false
}

//buildAllParamQuery kitty模型格式化所有param的参数。 如果join链接，参数为输出结果的structs
func (s *Structs) buildAllParamQuery() []*fieldQryFormat {
	query := []*fieldQryFormat{}
	for _, field := range s.Fields() {
		bindParam := "param:" //like param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !isNil(field) {
			if bindParam = GetSub(k, "param"); strings.Contains(bindParam, ".") {
				bindField := strings.Split(bindParam, ".")
				if q := formatQryParam(field); q != nil {
					q.model = strcase.ToSnake(bindField[0])     // bind model name
					q.fname = strcase.ToSnake(field.Name())     // structs field name
					q.bindfield = strcase.ToSnake(bindField[1]) // bind model field
					if strings.Contains(k, "condition") {
						q.withCondition = true
					}
					if strings.Contains(k, "ORDER") {
						q.order = true
					}
					query = append(query, q)
				}
			}
		}
	}
	return query
}

// BuildFormQuery ...生成有关model的全部param， 返回查询结构数组， 用于where 或者 join查询的ON
// 但当param被附加声明format后，并不返回。此操作可能为sum count 等聚合操作， 后续中为having。
func (s *Structs) buildFormQuery(model string) []*fieldQryFormat {
	withModel := strcase.ToSnake(model)
	query := []*fieldQryFormat{}
	for _, field := range s.Fields() {
		bindParam := "param:" + withModel + "." //param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !isNil(field) {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			if q := formatQryParam(field); q != nil {
				fname := strcase.ToSnake(bindField)
				q.bindfield = fname
				q.fname = field.Name()
				q.model = model
				if strings.Contains(k, "condition") {
					q.withCondition = true
				}
				if strings.Contains(k, "ORDER") {
					q.order = true
				}
				query = append(query, q)
			}
		}
	}
	return query
}

func (s *Structs) nameAs(names map[string][]string) {
	//	names := make()
	var f1 = func(typeKind FieldTypeAndKind, k string, names map[string][]string) {
		if typeKind.KindOfField == reflect.Struct ||
			typeKind.KindOfField == reflect.Slice && DereferenceType(typeKind.TypeOfField.Elem()).Kind() == reflect.Struct {
			bindfields := GetSub(k, "bind")
			if strings.Contains(bindfields, ".") {
				bindfields = strings.Split(bindfields, ".")[1]

				if v := strings.Split(bindfields, ","); bindfields != "*" && len(v) > 0 {
					names[typeKind.ModelName] = v
				}
			}
		}
	}
	for _, field := range s.Fields() {
		k := field.Tag("kitty")
		if len(k) > 0 && strings.Contains(k, "bind:") {
			typeKind := TypeKind(field)
			if strings.Contains(k, "bindresult") {
				if strings.Contains(k, fmt.Sprintf("bind:%s", strcase.ToSnake(typeKind.ModelName))) {
					f1(typeKind, k, names)
				} else {
					ss := typeKind.Create()
					ss.nameAs(names)
				}
			} else {
				f1(typeKind, k, names)
			}
		}
	}
	//	return names
}

// TypeKind 。。。
func TypeKind(field *structs.Field) FieldTypeAndKind {
	TypeKind := FieldTypeAndKind{}

	rt := DereferenceType(reflect.TypeOf(field.Value()))
	TypeKind.TypeOfField = rt
	TypeKind.KindOfField = rt.Kind()

	if rt.Kind() == reflect.Slice {
		rt = DereferenceType(rt.Elem())
		if rt.Kind() == reflect.Struct {
			TypeKind.ModelName = rt.Name()
		}
	} else if rt.Kind() == reflect.Struct {
		TypeKind.ModelName = rt.Name()
	} else {
		TypeKind.ModelName = field.Name()
	}

	typ := reflect2.TypeOf(field.Value())

	if typ.Kind() == reflect.Struct {
		TypeKind.t2 = typ
	} else if typ.Kind() == reflect.Ptr {
		ptrType := typ.(*reflect2.UnsafePtrType)
		if ptrType.Elem().Kind() == reflect.Struct {
			TypeKind.t2 = ptrType.Elem()
		}
	} else if typ.Kind() == reflect.Slice {
		sliceType := typ.(*reflect2.UnsafeSliceType)
		elemType := sliceType.Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.(*reflect2.UnsafePtrType).Elem()
		}
		if elemType.Kind() == reflect.Struct {
			TypeKind.t2 = elemType
		}

	}

	return TypeKind
}

// formatQryParam 转成 形如 Where("name IN (?)", []string{"jinzhu", "jinzhu 2"})
// having 需要完全的字段匹配
func formatQryParam(field *structs.Field) *fieldQryFormat {
	typeKind := TypeKind(field)
	operator := "="
	if k := field.Tag("kitty"); strings.Contains(k, "operator") {
		operator = GetSub(k, "operator")
	}

	if typeKind.KindOfField == reflect.Struct {
		return nil
	}
	singleValue := DereferenceValue(reflect.ValueOf(field.Value()))
	if typeKind.KindOfField == reflect.Slice {
		return &fieldQryFormat{operator: "IN (?)", value: []interface{}{singleValue.Interface()}}
	} else if typeKind.KindOfField == reflect.Interface {
		// 碰到这个类型，为gorm的expr
		return &fieldQryFormat{operator: fmt.Sprintf("%s (?)", operator), value: []interface{}{field.Value()}}
	}
	return &fieldQryFormat{operator: fmt.Sprintf("%s ?", operator), value: []interface{}{singleValue.Interface()}}
}

// Copy Structs
func (s *Structs) Copy(src *Structs) {
	for _, f := range s.Fields() {
		// Name string `alias:"" getter:"set(xxx(f('aaabcd')))"`
		fname := f.Name()

		var setValue = func(name string) bool {
			if sf, ok := src.FieldOk(name); ok {
				s.SetFieldValue(f, sf.Value())
				return true
			}
			return false
		}

		bok := false
		if alias := f.Tag("alias"); len(alias) > 0 {
			v := strings.Split(alias, ",")
			for _, field := range v {
				if setValue(field) {
					bok = true
					break
				}
			}
		}
		if !bok {
			if !setValue(fname) {
				if get := f.Tag("getter"); len(get) > 0 {
					Eval(s, nil, f, get)
				}
			}
		}
	}
}

// FormatQryField for test
func FormatQryField(field *structs.Field) string {
	return formatQryParam(field).operator
}
