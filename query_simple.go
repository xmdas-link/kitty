package kitty

import (
	"fmt"
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
	qryParams    []*fieldQryFormat
}

func (q *simpleQuery) create() (interface{}, error) {
	modelName := strcase.ToSnake(q.Result.Name())

	qryformats := q.qryParams
	if len(qryformats) == 0 { // 特别指定不从参数获取
		qryformats = append(qryformats, q.ModelStructs.buildFormQuery(modelName)...)
	}

	for _, qry := range qryformats {
		if f, ok := q.Result.FieldOk(ToCamel(qry.bindfield)); ok {
			if err := q.Result.SetFieldValue(f, qry.value[0]); err != nil {
				return nil, err
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

	qryformats := q.qryParams
	if len(qryformats) == 0 { // 特别指定不从参数获取
		qryformats = append(qryformats, q.ModelStructs.buildFormQuery(modelName)...)
	}

	for _, qry := range qryformats {
		if qry.withCondition || ToCamel(qry.bindfield) == "ID" {
			whereCount++
			if str := qry.nullExpr(); len(str) > 0 {
				tx = tx.Where(str)
			} else if g := qry.gormExpr(); g != nil {
				tx = tx.Where("?", g)
			} else {
				w := qry.whereExpr()
				tx = tx.Where(w, qry.value...)
			}
		} else if f, ok := q.Result.FieldOk(ToCamel(qry.bindfield)); ok {
			if str := qry.nullExpr(); len(str) > 0 {
				updates[qry.bindfield] = nil
			} else {
				updates[qry.bindfield] = qry.value[0]
				switch f.Value().(type) {
				case *time.Time, time.Time:
					if err := q.Result.SetFieldValue(f, qry.value[0]); err != nil {
						return err
					}
					updates[qry.bindfield] = f.Value()
				}
			}
		} else {
			return fmt.Errorf("%s field error %s", modelName, qry.bindfield)
		}
	}

	if whereCount == 0 {
		return fmt.Errorf("unable update %s, missing query condition", modelName)
	}

	if len(updates) > 0 {
		tx = tx.Updates(updates)
		q.search.ReturnCount = int(tx.RowsAffected)
		for k, v := range updates {
			if f, ok := q.Result.FieldOk(ToCamel(k)); ok {
				q.Result.SetFieldValue(f, v)
			}
		}
	}

	if err := tx.Error; err != nil {
		return err
	}

	for _, v := range q.Next {
		if tx.RowsAffected == 1 {
			if err := v.update(); err != nil {
				return err
			}
		}
	}
	return nil
}

/*
	switch f.Value().(type) {
	case *time.Time:
		if v, ok := resvalue.(string); ok {
			if len(v) == 0 {
				updates[qry.bindfield] = nil
				continue
			}
		}
		VK := reflect.ValueOf(resvalue)
		if VK.Kind() >= reflect.Int && VK.Kind() <= reflect.Float64 {
			v := VK.Convert(reflect.TypeOf(float64(0))).Interface().(float64)
			if v == 0 {
				updates[qry.bindfield] = nil
				continue
			}
		}
		if err := q.Result.SetFieldValue(f, resvalue); err != nil {
			return err
		}
		resvalue = DereferenceValue(reflect.ValueOf(f.Value()))
	case time.Time:
		if err := q.Result.SetFieldValue(f, resvalue); err != nil {
			return err
		}
		resvalue = DereferenceValue(reflect.ValueOf(f.Value()))
	}
*/
/*
	if DereferenceType(reflect.TypeOf(resvalue)).Kind() == reflect.Struct {
		if err := f.Set(resvalue); err != nil {
			return err
		}
		updateWithStrs = true
	} else {
		updates[qry.bindfield] = resvalue
	}
*/
