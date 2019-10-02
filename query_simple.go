package kitty

import (
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

// simpleQuery 单表查询更新创建
type simpleQuery struct {
	db           *gorm.DB
	search       *SearchCondition
	ModelStructs *Structs
	Result       *Structs
	Next         []*simpleQuery
}

func (q *simpleQuery) create() (interface{}, error) {
	modelName := q.Result.Name()

	for _, f := range q.ModelStructs.Fields() {
		if k := f.Tag("kitty"); strings.Contains(k, "param:") && !f.IsZero() {
			bindfield := GetSub(k, "param")
			if v := strings.Split(bindfield, "."); len(v) == 2 && ToCamel(v[0]) == modelName {
				field := ToCamel(v[1])
				if err := q.Result.SetFieldValue(q.Result.Field(field), f.Value()); err != nil {
					return nil, err
				}
			}
		}
	}
	tx := q.db.Create(q.Result.raw)

	if err := tx.Error; err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 {
				// solve the duplicate key error.
				return nil, fmt.Errorf("duplicate key, error: %s", mysqlErr)
			}
		}
		return nil, err
	}
	q.search.ReturnCount = int(tx.RowsAffected)

	for _, v := range q.Next {
		if tx.RowsAffected == 1 {
			id := q.Result.Field("ID").Value()
			foreignField := modelName + "ID" // ProductID-> product_id
			if f, ok := v.Result.FieldOk(foreignField); ok {
				f.Set(id)
			}
			if _, err := v.create(); err != nil {
				return nil, err
			}
		}
	}

	return q.Result.raw, nil
}

func (q *simpleQuery) update() error {

	whereCount := 0
	modelName := q.Result.Name()
	qryformat := q.ModelStructs.buildFormParamQuery(modelName, "ID")
	tx := q.db.Model(q.Result.raw)
	if qryformat != nil {
		//	return fmt.Errorf("unable update %s, not found param:id ", modelName)
		w := fmt.Sprintf("ID %s", qryformat.field)
		tx = tx.Where(w, qryformat.value...)
		whereCount++
	}

	for _, f := range q.ModelStructs.Fields() {
		if k := f.Tag("kitty"); strings.Contains(k, "param:") && !strings.Contains(k, "condition") && !f.IsZero() {
			bindfield := GetSub(k, "param")
			if v := strings.Split(bindfield, "."); len(v) == 2 && ToCamel(v[0]) == modelName {
				field := ToCamel(v[1])
				if field == "ID" {
					continue
				}
				if err := q.Result.SetFieldValue(q.Result.Field(field), f.Value()); err != nil {
					return err
				}
			}
		}
	}
	for _, f := range q.ModelStructs.Fields() {
		if k := f.Tag("kitty"); strings.Contains(k, "param:") && strings.Contains(k, "condition") && !f.IsZero() {
			bindfield := GetSub(k, "param")
			if v := strings.Split(bindfield, "."); len(v) == 2 && ToCamel(v[0]) == modelName {
				field := ToCamel(v[1])
				qryformat := q.ModelStructs.buildFormParamQueryCondition(modelName, field)
				w := fmt.Sprintf("%s %s", v[1], qryformat.field)
				tx = tx.Where(w, qryformat.value...)
				whereCount++
			}
		}
	}
	if whereCount == 0 {
		return fmt.Errorf("unable update %s, where condition is needed", modelName)
	}
	tx = tx.Update(q.Result.raw)

	if err := tx.Error; err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok {
			if mysqlErr.Number == 1062 {
				// solve the duplicate key error.
				return fmt.Errorf("duplicate key, error: %s", mysqlErr)
			}
		}
		return err
	}
	q.search.ReturnCount = int(tx.RowsAffected)

	for _, v := range q.Next {
		if tx.RowsAffected == 1 {
			if err := v.update(); err != nil {
				return err
			}
		}
	}
	return nil
}
