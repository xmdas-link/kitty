package kitty

import (
	"reflect"

	"github.com/jinzhu/gorm"
)

// query 单表查询
type query struct {
	db           *gorm.DB
	fieldselect  string
	search       *SearchCondition
	ModelStructs *Structs
	queryString  []*fieldQryFormat //参数
	queryStruct  interface{}       //参数
}

func (q *query) prepare() *gorm.DB {
	tx := q.db
	if len(q.fieldselect) > 0 {
		tx = tx.Select(q.fieldselect)
	}
	tx = tx.Model(q.ModelStructs.raw)
	for _, v := range q.queryString {
		tx = tx.Where(v.whereExpr(), v.value...)
	}
	if q.queryStruct != nil {
		tx = tx.Where(q.queryStruct)
	}
	return tx
}

func (q *query) one() (interface{}, error) {
	tx := q.prepare()
	/*
		if result != nil {
			rows, err := tx.Rows()
			if err != nil {
				return nil, err
			}
			if !rows.Next() {
				return nil, nil
			}
			if err = tx.ScanRows(rows, result); err != nil {
				return nil, err
			}
			q.search.ReturnCount = 1
			return result, nil
		}*/
	if !tx.First(q.ModelStructs.raw).RecordNotFound() {
		q.search.ReturnCount = 1
		return q.ModelStructs.raw, nil
	}
	return nil, nil
}

func (q *query) multi() (interface{}, error) {
	tx := q.prepare()

	scan := true
	//if result == nil {
	objValue := makeSlice(reflect.TypeOf(q.ModelStructs.raw), 0)
	result := objValue.Interface()
	scan = false
	//	} else {
	//		rv := reflect.ValueOf(result)
	//		objValue := makeSlice(reflect.TypeOf(rv.Elem().Interface()), 0)
	//		objValue := makeSlice(reflect.TypeOf(result), 0)
	//		result = objValue.Interface()
	//	}

	return pages(tx, q.search, result, scan)
}
