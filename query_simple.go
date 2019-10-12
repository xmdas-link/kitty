package kitty

import (
	"fmt"
	"reflect"
	"time"

	"github.com/iancoleman/strcase"

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
	modelName := strcase.ToSnake(q.Result.Name())

	qryformats := q.ModelStructs.buildAllParamQuery()
	for _, qry := range qryformats {
		if modelName == qry.model {
			if f, ok := q.Result.FieldOk(ToCamel(qry.bindfield)); ok {
				if err := q.Result.SetFieldValue(f, qry.value[0]); err != nil {
					return nil, err
				}
			}
		}
	}
	tx := q.db.Create(q.Result.raw)

	if err := tx.Error; err != nil {
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
	modelName := strcase.ToSnake(q.Result.Name())
	tx := q.db.Model(q.Result.raw)

	updates := make(map[string]interface{})

	qryformats := q.ModelStructs.buildAllParamQuery()
	for _, qry := range qryformats {
		if modelName == qry.model {
			if qry.withCondition || ToCamel(qry.bindfield) == "ID" {
				whereCount++
				w := qry.whereExpr()
				tx = tx.Where(w, qry.value...)
			} else if f, ok := q.Result.FieldOk(ToCamel(qry.bindfield)); ok {
				if err := q.Result.SetFieldValue(f, qry.value[0]); err != nil {
					return err
				}
				switch f.Value().(type) {
				case *time.Time:
					if v, ok := qry.value[0].(string); ok {
						if len(v) == 0 {
							updates[qry.bindfield] = nil
							continue
						}
					}
					VK := reflect.ValueOf(qry.value[0])
					if VK.Kind() >= reflect.Int && VK.Kind() <= reflect.Float64 {
						v := VK.Convert(reflect.TypeOf(float64(0))).Interface().(float64)
						if v == 0 {
							updates[qry.bindfield] = nil
							continue
						}
					}
				}
				updates[qry.bindfield] = f.Value()
			}
		}
	}
	if whereCount == 0 {
		return fmt.Errorf("unable update %s, missing query condition", modelName)
	}
	//tx = tx.Update(q.Result.raw)
	tx = tx.Updates(updates)

	if err := tx.Error; err != nil {
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
