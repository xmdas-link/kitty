package kitty

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/now"

	"github.com/fatih/structs"
)

// Structs .
type Structs struct {
	*structs.Struct
	raw interface{}
	//strTypes map[string]reflect2.Type
}

// FieldTypeAndKind 字段类型，模型名称
type FieldTypeAndKind struct {
	ModelName   string       //模型名称
	KindOfField reflect.Kind //类型  struct
	TypeOfField reflect.Type //类型
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
	format        string        // like format:sum($)
}

func (f *fieldQryFormat) whereExpr() string {
	return fmt.Sprintf("%s %s", f.bindfield, f.operator)
}

// CreateModelStructs ...
func CreateModelStructs(v interface{}) *Structs {
	return &Structs{structs.New(v), v}
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
	case time.Time, *time.Time:
		if VK == reflect.String {
			//stamp, err := time.ParseInLocation("2006-01-02 15:04:05", rv.Interface().(string), time.Local)
			stamp := now.New(time.Now().UTC()).MustParse(rv.Interface().(string))
			return f1(reflect.ValueOf(stamp))
		}
		if VK >= reflect.Int && VK <= reflect.Float64 {
			str := fmt.Sprintf("%v", rv)
			x, _ := strconv.ParseInt(str, 10, 64)
			stamp := time.Unix(x, 0)
			return f1(reflect.ValueOf(stamp))
		}
		return fmt.Errorf("%s: %v 时间格式错误", f.Name(), rv)
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
			panic("")
		}
		field := s.Field(ToCamel(trimSpace(p[0])))
		value, err := src.getValue(trimSpace(p[1]))
		if err != nil {
			return err
		}
		if value != nil {
			if err := s.SetFieldValue(field, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// param可能是字段，也可能普通字符串. 如果非字段，则返回该param
// param可能包含及联，则遇到slice的时候，默认读取第一个。
// params like: name=username id=1 id=product.id id=product.data.id
// name=function('abcd')
func (s *Structs) getValue(param string) (interface{}, error) {
	if strings.Contains(param, ".") {
		vv := strings.Split(param, ".")

		fieldName := vv[0]
		sliceIdx := -1
		if i := strings.Index(fieldName, "["); i > 0 {
			b := strings.Index(fieldName, "]")
			str := fieldName[i+1 : b]
			fieldName = fieldName[:i]
			idx, _ := strconv.ParseInt(str, 10, 64)
			sliceIdx = int(idx)
		}
		field := s.Field(ToCamel(fieldName))
		if field.IsZero() {
			return nil, nil
		}
		fieldvalue := field.Value()
		tk := TypeKind(field)
		if tk.KindOfField != reflect.Slice && tk.KindOfField != reflect.Struct {
			panic("")
		}
		if tk.KindOfField == reflect.Slice {
			slicevalue := DereferenceValue(reflect.ValueOf(fieldvalue))
			if slicevalue.Len() < sliceIdx {
				return nil, fmt.Errorf("slice idx overflow %s", field.Name())
			}
			fieldvalue = slicevalue.Index(sliceIdx).Interface()
		}
		ss := CreateModelStructs(fieldvalue)
		p := strings.Join(vv[1:], ".")
		return ss.getValue(p)
	}
	param = strings.ReplaceAll(param, "`", "")
	if f, ok := s.FieldOk(ToCamel(param)); ok {
		if f.IsZero() {
			return nil, nil
		}
		tk := TypeKind(f)
		if tk.KindOfField == reflect.Interface {
			return reflect.ValueOf(f.Value()).Elem().Interface(), nil
		}
		return DereferenceValue(reflect.ValueOf(f.Value())).Interface(), nil
	}
	return param, nil
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
			ss := CreateModel(modelName) //NewModelStruct(modelName)
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
		if len(k) > 0 && strings.Contains(k, "param") {
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

//buildAllParamQuery kitty模型格式化所有param的参数。 如果join链接，参数为输出结果的structs
func (s *Structs) buildAllParamQuery() []*fieldQryFormat {
	query := []*fieldQryFormat{}
	for _, field := range s.Fields() {
		bindParam := "param:" //like param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !field.IsZero() {
			if bindParam = GetSub(k, "param"); strings.Contains(bindParam, ".") {
				bindField := strings.Split(bindParam, ".")
				if q := formatQryParam(field); q != nil {
					q.model = strcase.ToSnake(bindField[0])     // bind model name
					q.fname = strcase.ToSnake(field.Name())     // structs field name
					q.bindfield = strcase.ToSnake(bindField[1]) // bind model field

					if strings.Contains(k, "condition") {
						q.withCondition = true
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
func (s *Structs) buildFormQuery(tblname, model string) []*fieldQryFormat {
	withModel := strcase.ToSnake(model)
	query := []*fieldQryFormat{}
	for _, field := range s.Fields() {
		bindParam := "param:" + withModel + "." //param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !field.IsZero() {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			if q := formatQryParam(field); q != nil {
				fname := strcase.ToSnake(bindField)
				q.operator = fmt.Sprintf("%s %s", fname, q.operator)
				if len(tblname) > 0 {
					q.operator = fmt.Sprintf("%s.%s", tblname, q.operator)
				}
				query = append(query, q)
			}
		}
	}
	return query
}

// BuildFormFieldQuery ....仅是模型的某个字段的查询格式化。用于以join方式的where.
func (s *Structs) buildFormFieldQuery(fieldname string) *fieldQryFormat {
	FieldName := ToCamel(fieldname)
	if field, ok := s.FieldOk(FieldName); ok && !field.IsZero() {
		return formatQryParam(field)
	}
	return nil
}

// BuildFormParamQuery ....生成模型的指定字段的查询格式化参数。
func (s *Structs) buildFormParamQuery(modelname, fieldname string) *fieldQryFormat {
	withModel := strcase.ToSnake(modelname)
	for _, field := range s.Fields() {
		bindParam := "param:" + withModel //param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !field.IsZero() {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			fname := strcase.ToSnake(fieldname)
			if bindField == fname {
				return formatQryParam(field)
			}
		}
	}
	return nil
}

// buildFormParamQueryCondition 比 BuildFormParamQuery 多了一个condition约束。
// 在更新的时候，condition作为where, 非update选型。
func (s *Structs) buildFormParamQueryCondition(modelname, fieldname string) *fieldQryFormat {
	withModel := strcase.ToSnake(modelname)
	for _, field := range s.Fields() {
		bindParam := "param:" + withModel //param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && strings.Contains(k, "condition") && !field.IsZero() {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			fname := strcase.ToSnake(fieldname)
			if bindField == fname {
				if strcase.ToSnake(field.Name()) == fname {
					return formatQryParam(field)
				}
				//特殊的情况：当having的时候，form参数绑定的字段不是model的字段，而是兄弟字段
				return formatQryParam(field)
			}
		}
	}
	return nil
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
					ss := CreateModel(typeKind.ModelName) //NewModelStruct(typeKind.ModelName)
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
		singleValue = reflect.ValueOf(field.Value()).Elem()
	}
	return &fieldQryFormat{operator: fmt.Sprintf("%s ?", operator), value: []interface{}{singleValue.Interface()}}
}

/*
	var convert = func(k reflect.Kind, v interface{}) interface{} {
		if k >= reflect.Int && k <= reflect.Float64 && reflect.ValueOf(v).Kind() == reflect.String {
			x, _ := strconv.ParseFloat(v.(string), 64)
			return x
		}
		return v
	}

	if singleValue.Kind() == reflect.String {
		str := singleValue.Interface().(string)
		str = strings.TrimSpace(str)

		// 模糊查询
		fuzzyKey := []string{"%", "_", "["}
		for _, v := range fuzzyKey {
			if strings.Contains(str, v) {
				return &fieldQryFormat{operator: "LIKE ?", value: []interface{}{singleValue.Interface()}}
			}
		}

		//查询绑定的模型字段类型
		k := field.Tag("kitty")
		if !strings.Contains(k, "param:") {
			return &fieldQryFormat{operator: "= ?", value: []interface{}{singleValue.Interface()}}
		}
		bindField := strings.Split(GetSub(k, "param"), ".") //user.name
		s := CreateModel(bindField[0])
		value := s.Field(ToCamel(bindField[1])).Value()

		switch value.(type) {
		case time.Time, *time.Time:
		default:
			tk := DereferenceValue(reflect.ValueOf(value))
			kd = tk.Kind()
			if tk.Kind() == reflect.String {
				return &fieldQryFormat{operator: "= ?", value: []interface{}{singleValue.Interface()}}
			}
		}

		if strings.Count(str, "..") == 1 {
			v := strings.Split(str, "..")
			return &fieldQryFormat{operator: "BETWEEN ? AND ?", value: []interface{}{convert(kd, v[0]), convert(kd, v[1])}}
		}

		if strings.Contains(str, ",") {
			v := strings.Split(str, ",")
			len := len(v)
			if len >= 2 {
				if v[0] == "" {
					operator = "<="
					singleValue = reflect.ValueOf(v[1])
				} else if v[1] == "" {
					operator = ">="
					singleValue = reflect.ValueOf(v[0])
				} else {
					var vv []interface{}
					for _, v1 := range v {
						vv = append(vv, convert(kd, v1))
					}
					return &fieldQryFormat{operator: "IN (?)", value: []interface{}{v}}
				}
			}
		}
	}
	return &fieldQryFormat{operator: fmt.Sprintf("%s ?", operator), value: []interface{}{convert(kd, singleValue.Interface())}}
*/

// FormatQryField for test
func FormatQryField(field *structs.Field) string {
	return formatQryParam(field).operator
}
