package kitty

import (
	"fmt"
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
	raw      interface{}
	strTypes map[string]reflect2.Type
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

type fieldQryFormat struct {
	field string
	v     []interface{}
}

// createModelStructs ...
func createModelStructs(v interface{}) *Structs {
	s := &Structs{structs.New(v), v, make(map[string]reflect2.Type)}
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
			s.strTypes[strcase.ToSnake(nativeType.Name())] = type2
		}
	}
	return s
}

func (s *Structs) createObj(name string) interface{} {
	if v := s.strTypes[strcase.ToSnake(name)]; v != nil {
		return v.New()
	}
	panic("")
}

func (s *Structs) createModelStructs(name string) *Structs {
	return createModelStructs(s.createObj(name))
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
	FK := f.Kind()

	rv := DereferenceValue(reflect.ValueOf(value))
	VK := rv.Kind()

	RealType := reflect.TypeOf(f.Value())
	FT := DereferenceType(RealType)
	FK = FT.Kind()

	// 同一类型 ， 暂不在支持 map，结构体，切片
	if VK == FK && FK != reflect.Struct && FK != reflect.Map && FK != reflect.Slice {
		if RealType.Kind() != reflect.Ptr {
			return f.Set(rv.Interface())
		}
		return f.Set(ptr(rv).Interface())
	}
	var x interface{}

	if VK == reflect.String {
		switch f.Value().(type) {
		case time.Time:
			stamp, err := time.ParseInLocation("2006-01-02 15:04:05", rv.Interface().(string), time.Local)
			if err == nil {
				f.Set(stamp)
				return nil
			}
			return fmt.Errorf("%s :时间格式错误, 正确的格式: 2006-01-02 15:04:05", rv.Interface().(string))
		}
		switch FK {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			x, _ = strconv.ParseInt(rv.Interface().(string), 10, 64)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			x, _ = strconv.ParseUint(rv.Interface().(string), 10, 64)
		case reflect.Float32, reflect.Float64:
			x, _ = strconv.ParseFloat(rv.Interface().(string), 64)
		default:
			return fmt.Errorf("wrong kind. got: %s want: %s", VK, FK)
		}
	} else if FK >= reflect.Int && FK <= reflect.Float64 && VK >= reflect.Int && VK <= reflect.Float64 { // 同为整型
		x = rv.Interface()
	} else if FK == reflect.String && VK >= reflect.Int && VK <= reflect.Float64 {
		f.Set(fmt.Sprintf("%v", rv))
		return nil
	}

	if x != nil {
		//	FT := reflect.TypeOf(f.Value())
		v1 := reflect.ValueOf(x).Convert(FT) //.Interface()
		if RealType.Kind() != reflect.Ptr {
			f.Set(v1.Interface())
		} else {
			f.Set(ptr(v1).Interface())
		}
		return nil
	}

	return fmt.Errorf("Not Support kind. %s want: %s", VK, FK)
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

func (s *Structs) setStructValue(field *structs.Field, values map[string]interface{}) error {
	for k := range values {
		if _, ok := field.FieldOk(ToCamel(k)); !ok {
			return fmt.Errorf("field error %v", k)
		}
	}
	for _, f := range field.Fields() {
		name := f.Name()
		if v, ok := values[name]; ok {
			if err := s.SetFieldValue(f, v); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetID ...
func (s *Structs) SetID(v uint64) {
	field := s.Field("ID")
	s.SetFieldValue(field, v)
}

// GetStructFieldInfo fieldname (elem) must struct
func (s *Structs) GetStructFieldInfo(fieldname string) (fi StructFieldInfo, err error) {
	if field, ok := s.FieldOk(fieldname); ok {
		fi.TypeKind = (&FormField{field}).TypeAndKind()

		if len(fi.TypeKind.ModelName) == 0 {
			return fi, fmt.Errorf("invalid field %s, is not a struct", fieldname)

		}
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
			fi.ForeignKey = ToCamel(fieldname) + "ID"
			testNewForeignKey = true
		}
		//查找默认的字段 Email-> EmailId
		// with belongs to
		if _, ok := s.FieldOk(fi.ForeignKey); ok {
			fi.Relationship = "belongs_to"
		} else {
			if testNewForeignKey {
				fi.ForeignKey = s.Name() + "ID" // with has one...
			}
			ss := s.createModelStructs(fi.TypeKind.ModelName) //NewModelStruct(fi.TypeKind.ModelName)
			if _, ok := ss.FieldOk(fi.ForeignKey); ok {
				if fi.TypeKind.KindOfField == reflect.Struct {
					fi.Relationship = "has_one"
				} else if fi.TypeKind.KindOfField == reflect.Slice {
					fi.Relationship = "has_many"
				}
			} else {
				fi.Relationship = "nothing"
			}

		}
	}
	return fi, nil
}

/*
// FillStructField fieldname must struct
func (s *Structs) FillStructField(fieldname string, values map[string][]interface{}) error {
	fi, err := s.GetStructFieldInfo(fieldname)

	if err != nil {
		return err
	}

	field := s.Field(fieldname)
	ss := NewModelStruct(fi.TypeKind.ModelName)

	var objVaule interface{}
	if fi.TypeKind.KindOfField == reflect.Struct {
		for field, v := range values {
			field = ToCamel(field)
			ss.SetFieldValue(ss.Field(field), v[0])
		}
		objVaule = reflect.ValueOf(ss.raw).Interface()
	} else if fi.TypeKind.KindOfField == reflect.Slice {
		// 检查有几个
		lenSlice := 0
		for _, v := range values {
			lenSlice = len(v)
			break
		}

		slice := makeSlice(fi.TypeKind.TypeOfField, lenSlice) // []*Email []Email
		elemField := fi.TypeKind.TypeOfField.Elem()           // like *Email Email
		for i := 0; i < lenSlice; i++ {
			for field, v := range values {
				field = ToCamel(field)
				ss.SetFieldValue(ss.Field(field), v[i])
			}
			vss := reflect.ValueOf(ss.raw)
			for vss.Kind() != elemField.Kind() {
				vss = vss.Elem()
			}
			slice.Elem().Index(i).Set(vss)
		}
		objVaule = slice.Interface()
	}

	vObj := reflect.ValueOf(objVaule)
	if field.Kind() != reflect.Ptr && vObj.Kind() == reflect.Ptr {
		vObj = vObj.Elem()
	}

	return field.Set(vObj.Interface())

}
*/
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
			ss := s.createModelStructs(modelName) //NewModelStruct(modelName)
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
				if err := s.SetFieldValue(field, formvalue[0]); err != nil {
					return err
				}
			}
		}

	}
	return nil
}

// BuildFormQuery ...
func (s *Structs) buildFormQuery(tblname, model string) []*fieldQryFormat {
	withModel := strcase.ToSnake(model)
	//	ss := NewModelStruct(withModel)
	query := []*fieldQryFormat{}
	for _, field := range s.Fields() {
		bindParam := "param:" + withModel + "." //param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !field.IsZero() {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			if q := (&FormField{field}).toQuery(); q != nil {
				fname := strcase.ToSnake(bindField)
				if len(tblname) > 0 {
					q.field = fmt.Sprintf("%s.%s %s", tblname, fname, q.field)
					//query = append(query, fmt.Sprintf("%s.%s %s", tblname, fname, q))
				} else {
					q.field = fmt.Sprintf("%s %s", fname, q.field)
					//query = append(query, fmt.Sprintf("%s %s", fname, q))
				}
				query = append(query, q)
			}
		}
	}
	return query
}

// BuildFormFieldQuery ....
func (s *Structs) buildFormFieldQuery(fieldname string) *fieldQryFormat {
	FieldName := ToCamel(fieldname)
	if field, ok := s.FieldOk(FieldName); ok && !field.IsZero() {
		return (&FormField{field}).toQuery()
	}
	return nil
}

// BuildFormParamQuery ....
func (s *Structs) buildFormParamQuery(modelname, fieldname string) *fieldQryFormat {
	withModel := strcase.ToSnake(modelname)
	for _, field := range s.Fields() {
		bindParam := "param:" + withModel //param:order_item.order_id
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !field.IsZero() {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			fname := strcase.ToSnake(fieldname)
			if bindField == fname {
				if strcase.ToSnake(field.Name()) == fname {
					return (&FormField{field}).toQuery()
				}
				//特殊的情况：当having的时候，form参数绑定的字段不是model的字段，而是兄弟字段
				return (&FormField{field}).toQuery()
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
			typeKind := (&FormField{field}).TypeAndKind()
			if strings.Contains(k, "bindresult") {
				if strings.Contains(k, fmt.Sprintf("bind:%s", strcase.ToSnake(typeKind.ModelName))) {
					f1(typeKind, k, names)
				} else {
					ss := s.createModelStructs(typeKind.ModelName) //NewModelStruct(typeKind.ModelName)
					ss.nameAs(names)
				}

			} else {
				f1(typeKind, k, names)
			}
		}
	}
	//	return names
}

// FormField ...
type FormField struct {
	*structs.Field
}

// TypeAndKind 。。。
func (field *FormField) TypeAndKind() FieldTypeAndKind {
	TypeKind := FieldTypeAndKind{}

	rt := DereferenceType(reflect.TypeOf(field.Value()))
	TypeKind.TypeOfField = rt

	if rt.Kind() == reflect.Slice {
		TypeKind.KindOfField = reflect.Slice
		rt = DereferenceType(rt.Elem())
		if rt.Kind() == reflect.Struct {
			TypeKind.ModelName = rt.Name()
		}
	} else if rt.Kind() == reflect.Struct {
		TypeKind.KindOfField = reflect.Struct
		TypeKind.ModelName = rt.Name()
	} else {
		TypeKind.KindOfField = field.Kind()
		TypeKind.ModelName = field.Name()
	}
	return TypeKind
}

// ToQuery 转成 形如 Where("name IN (?)", []string{"jinzhu", "jinzhu 2"})
// having 需要完全的字段匹配
func (field *FormField) toQuery() *fieldQryFormat {
	typeKind := field.TypeAndKind()

	if typeKind.KindOfField == reflect.Struct {
		return nil
	}

	singleValue := DereferenceValue(reflect.ValueOf(field.Value()))

	compare := "="

	if singleValue.Kind() == reflect.String {
		s := singleValue.Interface().(string)
		if strings.Contains(s, ",") {
			v := strings.Split(s, ",")
			len := len(v)
			if len > 2 {
				return &fieldQryFormat{field: "IN (?)", v: []interface{}{v}}
			} else if len == 2 {
				if v[0] == "" {
					compare = "<="
					singleValue = reflect.ValueOf(v[1])
				} else if v[1] == "" {
					compare = ">="
					singleValue = reflect.ValueOf(v[0])
				} else {
					return &fieldQryFormat{field: "BETWEEN ? AND ?", v: []interface{}{v[0], v[1]}}
				}
			}
		}
	}
	return &fieldQryFormat{field: fmt.Sprintf("%s ?", compare), v: []interface{}{singleValue.Interface()}}
}
