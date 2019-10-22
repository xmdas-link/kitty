package kitty

import (
	"github.com/jinzhu/gorm"
)

// query 单表查询
type query struct {
	db           *gorm.DB
	fieldselect  string
	search       *SearchCondition
	ModelStructs *Structs
	queryString  []*fieldQryFormat //参数
	order        []*fieldQryFormat
	scan         bool //scan result
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
	for _, v := range q.order {
		tx = tx.Order(v.orderExpr())
	}
	return tx
}

func (q *query) one() (interface{}, error) {
	tx := q.prepare()
	if !tx.First(q.ModelStructs.raw).RecordNotFound() {
		q.search.ReturnCount = 1
		return q.ModelStructs.raw, nil
	}
	return nil, nil
}

func (q *query) multi() (interface{}, error) {
	//	tx := q.prepare()
	//	objValue := makeSlice(reflect.TypeOf(q.ModelStructs.raw), 0)
	//	result := objValue.Interface()
	return nil, nil //pages(tx, q.search, result, q.scan)
}
