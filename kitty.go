package kitty

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"
)

func raw(ms *Structs, cur *Structs, db *gorm.DB, str string) interface{} {
	expr := &expr{
		db:        db,
		s:         cur,
		functions: make(map[string]govaluate.ExpressionFunction),
		params:    make(map[string]interface{}),
		createM:   ms.createModel,
	}
	expr.init()

	if cur != nil {
		expr.params["s"] = cur.raw
		for _, f := range cur.Fields() {
			k := f.Tag("kitty")
			if getter := GetSub(k, "getter"); len(getter) > 0 {
				expr.f = f
				if err := expr.eval(getter); err != nil {
					return err
				}
			}
		}
	} else {
		expr.s = ms
	}

	type result struct {
		Result *interface{}
	}
	s := CreateModelStructs(&result{})
	f := s.Field("Result")
	expr.f = f
	expr.eval(str)
	return reflect.ValueOf(f.Value()).Elem().Interface()
}

type fieldBinding struct {
	ModelName      string // User Company
	TableName      string // users companies
	FieldName      string // UserName
	BindModelField string // Name
	Format         string // 字段格式化 strftime('%s',$);     -->结果：1525478400
	exprString     bool   //是一句内置函数表达式
	strs           *Structs
}

func (f *fieldBinding) selects(ms *Structs, db *gorm.DB) *fieldQryFormat {
	if f.exprString {
		return &fieldQryFormat{
			bindfield: fmt.Sprintf("(?) AS %s", strcase.ToSnake(f.FieldName)),
			value:     []interface{}{raw(ms, f.strs, db, f.BindModelField)},
		}
	}
	return &fieldQryFormat{
		bindfield: f.selectAs(),
	}
}

func (f *fieldBinding) selectAs() string {
	if f.exprString {
		return ""
	}
	if len(f.Format) > 0 {
		format := strings.Replace(f.Format, "$", f.tableWithFieldName(), -1)
		return fmt.Sprintf("%s AS %s", format, strcase.ToSnake(f.FieldName))
	}
	if f.BindModelField == "*" {
		return fmt.Sprintf("%s", f.tableWithFieldName())
	} else if strings.Contains(f.BindModelField, ",") { // multi field
		fields := strings.Split(f.BindModelField, ",")
		fieldsFormat := []string{}
		for _, field := range fields {
			fieldsFormat = append(fieldsFormat, fmt.Sprintf("%s.%s", f.TableName, field))
		}
		return strings.Join(fieldsFormat, ", ")
	} else if len(f.BindModelField) > 0 {
		return fmt.Sprintf("%s.%s", f.TableName, f.BindModelField)
	}
	return fmt.Sprintf("%s AS %s", f.tableWithFieldName(), strcase.ToSnake(f.FieldName))
}

func (f *fieldBinding) tableWithFieldName() string {
	// for kitty model . tablename is an empty string.
	if len(f.TableName) == 0 {
		return strcase.ToSnake(f.BindModelField)
	}
	return fmt.Sprintf("%s.%s", f.TableName, strcase.ToSnake(f.BindModelField))
}

func (f *fieldBinding) funcName() string { //sum(xx)
	if len(f.Format) > 0 {
		return strings.Replace(f.Format, "$", f.tableWithFieldName(), -1)
	}
	return f.tableWithFieldName()
}

// kitty Join字段的约束
type kitty struct {
	ModelName    string   // User Company
	FieldName    string   //
	TableName    string   // users companies
	Master       bool     // 主表
	JoinAction   string   // 连接方式 left / right / inner
	JoinTo       string   // 关联的模型
	Group        []string // group by a, b  需定义为输出的字段名称。
	structs      *Structs
	ModelStructs *Structs //form structs
}

// parse kitty:"bind:order_item.*;
func (j *kitty) parse(k, modelName, fieldName string) {
	j.Master = strings.Contains(k, "master")
	//join aciton
	j.JoinAction = strings.ToUpper(GetSub(k, "join")) + " JOIN"
	j.JoinTo = ToCamel(GetSub(k, "model"))
	if s := GetSub(k, "group"); len(s) > 0 {
		j.Group = strings.Split(s, ",")
	}
}
func (j *kitty) binding(k, modelName, fieldName string) *fieldBinding {
	binding := &fieldBinding{
		ModelName:      modelName,
		TableName:      j.TableName,
		Format:         GetSub(k, "format"),
		BindModelField: "*",
		FieldName:      strcase.ToSnake(fieldName),
		exprString:     false,
	}
	// bind:user.id,name,age
	// countSomething int bind:rds('','user.count(1)')
	if s := GetSub(k, "bind"); len(s) > 0 {
		if strings.Contains(s, "(") && strings.Contains(s, ")") {
			binding.BindModelField = s
			binding.exprString = true
		} else {
			modelField := strings.Split(s, ".")
			binding.BindModelField = modelField[1]
		}
	}
	return binding
}

func (j *kitty) groupBy() []string {
	return j.Group
}

// join gorm model . select * from users left join companies on companies.id =  users.id
func (j *kitty) joins(s *Structs, joinTo *kitty) *fieldQryFormat {
	join := fmt.Sprintf("%s %s", j.JoinAction, j.TableName) // left join companies
	where := []string{}
	if len(j.JoinTo) > 0 {
		if fi, err := s.GetRelationsWithModel(j.FieldName, joinTo.ModelName); err == nil {
			if fi.Relationship != "nothing" {
				associationKey := strcase.ToSnake(fi.ForeignKey)
				where = append(where, fmt.Sprintf("%s.%s = %s.%s",
					j.TableName,
					associationKey,
					joinTo.TableName,
					strcase.ToSnake(fi.AssociationForeignkey)))
			}
		}
	}

	params := []interface{}{}
	if query := s.buildFormQuery(j.ModelName); len(query) > 0 {
		for _, v := range query {
			v.bindfield = fmt.Sprintf("%s.%s", j.TableName, v.bindfield)
			if str := v.nullExpr(); len(str) > 0 {
				where = append(where, str)
			} else {
				where = append(where, v.whereExpr())
				params = append(params, v.value...)
			}
		}
	}

	qryformat := &fieldQryFormat{operator: fmt.Sprintf("%s ON %s", join, strings.Join(where, " AND ")), value: params}
	return qryformat
}

// join kitty model.
// select * from users left join (select count(1) from work_issues group by name) as tmp on tmp.name=users.name
func (j *kitty) joinKitty(s *Structs, joinTo *kitty, db *gorm.DB, ctx Context) *fieldQryFormat {
	// 构造kitty模型的所需参数
	for _, field := range s.Fields() {
		bindParam := "param:" + strcase.ToSnake(j.ModelName) + "." //param:order_item.*
		if k := field.Tag("kitty"); strings.Contains(k, bindParam) && !isNil(field) {
			bindField := strings.Split(GetSub(k, "param"), ".")[1]
			f := j.structs.Field(ToCamel(bindField))
			j.structs.SetFieldValue(f, field.Value())
		}
	}
	join := fmt.Sprintf("%s (?) AS %s", j.JoinAction, j.FieldName) // left join companies
	where := []string{}
	if len(j.JoinTo) > 0 {
		if fi, err := s.GetRelationsWithModel(j.FieldName, joinTo.ModelName); err == nil {
			if fi.Relationship != "nothing" {
				associationKey := strcase.ToSnake(fi.ForeignKey)
				where = append(where, fmt.Sprintf("%s.%s = %s.%s",
					j.FieldName,
					associationKey,
					joinTo.TableName,
					strcase.ToSnake(fi.AssociationForeignkey)))
			}
		}
	}
	q := newcrud(&config{
		strs:   j.structs,
		search: &SearchCondition{},
		db:     db,
		ctx:    ctx,
	})
	res, _ := q.queryExpr()
	params := []interface{}{res}
	qryformat := &fieldQryFormat{operator: fmt.Sprintf("%s ON %s", join, strings.Join(where, " AND ")), value: params}
	return qryformat
}
