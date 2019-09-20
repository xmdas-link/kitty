package kitty

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"
)

type fieldBinding struct {
	ModelName      string // User Company
	TableName      string // users companies
	FieldName      string // UserName
	BindModelField string // Name
	Func           string // count / sum
	Format         string // 字段格式化 strftime('%s',$);     -->结果：1525478400
	Having         bool
}

func (f *fieldBinding) selectAs() string {
	if len(f.Format) > 0 {
		format := strings.Replace(f.Format, "$", f.tableWithFieldName(), -1)
		return fmt.Sprintf("%s AS %s", format, strcase.ToSnake(f.FieldName))
	}
	if len(f.Func) > 0 {
		return fmt.Sprintf("%s(%s) AS %s", f.Func, f.tableWithFieldName(), strcase.ToSnake(f.FieldName))
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
	}
	return fmt.Sprintf("%s AS %s", f.tableWithFieldName(), strcase.ToSnake(f.FieldName))
}

func (f *fieldBinding) tableWithFieldName() string {
	return fmt.Sprintf("%s.%s", f.TableName, strcase.ToSnake(f.BindModelField))
}

func (f *fieldBinding) funcName() string { //sum(xx)
	if len(f.Func) > 0 {
		return fmt.Sprintf("%s(%s)", f.Func, f.tableWithFieldName())
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
func (j *kitty) parse(k, modelName, fieldName string, db *gorm.DB) *fieldBinding {
	if len(j.TableName) == 0 {
		j.ModelName = modelName
		j.FieldName = fieldName
		j.structs = CreateModel(modelName)                             //NewModelStruct(modelName)                                   // OrderItem
		j.TableName = db.NewScope(j.structs.raw).GetModelStruct().TableName(db) //order_items
	}

	if s := GetSub(k, "bind"); len(s) > 0 {
		modelField := strings.Split(s, ".")
		binding := &fieldBinding{
			ModelName:      modelName,
			TableName:      j.TableName,
			Func:           strings.ToUpper(GetSub(k, "func")),
			Format:         GetSub(k, "format"),
			BindModelField: modelField[1],
			FieldName:      strcase.ToSnake(fieldName),
			Having:         strings.Contains(k, "having"),
		}
		return binding
	}
	j.Master = strings.Contains(k, "master")
	//join aciton
	j.JoinAction = strings.ToUpper(GetSub(k, "join")) + " JOIN"
	j.JoinTo = ToCamel(GetSub(k, "model"))
	if s := GetSub(k, "group"); len(s) > 0 {
		j.Group = strings.Split(s, ",")
	}
	return nil
}

// model.name -> company_name
func (j *kitty) fieldName(bindField string) string {
	//	for _, v := range j.fieldBind {
	//		if v.BindModelField == bindField {
	//			return v.FieldName
	//		}
	//	}
	return fmt.Sprintf("%s.%s", j.TableName, bindField)
}

/*func (j *kitty) selectAs() []string {
	s := []string{}
	for _, bind := range j.fieldBind {
		s = append(s, bind.SelectAs())
	}
	return s
}*/
func (j *kitty) groupBy() []string {
	//	s := []string{}
	//	if len(j.Group) > 0 {
	//		for _, v := range j.Group {
	//			s = append(s, j.fieldName(v))
	//		}
	//	}
	//	return s
	return j.Group
}

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
	if query := s.buildFormQuery(j.TableName, j.ModelName); len(query) > 0 {
		for _, v := range query {
			where = append(where, v.field)
			params = append(params, v.v...)
		}
	}

	qryformat := &fieldQryFormat{field: fmt.Sprintf("%s ON %s", join, strings.Join(where, " AND ")), v: params}
	return qryformat
}
