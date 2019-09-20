package kitty

import (
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
)

// JoinQuery ... 联表查询
type joinQuery struct {
	db           *gorm.DB
	search       *SearchCondition
	ModelStructs *Structs
	TableName    string
	Selects      []string
	Joins        []*fieldQryFormat
	Where        []*fieldQryFormat
	GroupBy      []string
	Having       *fieldQryFormat
}

func (q *joinQuery) prepare() *gorm.DB {
	tx := q.db.Table(q.TableName).Select(q.Selects)
	for _, v := range q.Joins {
		tx = tx.Joins(v.field, v.v...)
	}
	for _, v := range q.Where {
		tx = tx.Where(v.field, v.v...)
	}
	//for _, v := range q.GroupBy {
	if len(q.GroupBy) > 0 {
		tx = tx.Group(strings.Join(q.GroupBy, ", "))
	}
	//}
	if q.Having != nil {
		tx = tx.Having(q.Having.field, q.Having.v...)
	}
	return tx
}
func (q *joinQuery) one() (interface{}, error) {
	tx := q.prepare()
	rows, err := tx.Rows()
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		return nil, nil
	}
	if err = tx.ScanRows(rows, q.ModelStructs.raw); err != nil {
		return nil, err
	}
	q.search.ReturnCount = 1
	return q.ModelStructs.raw, nil
}

func (q *joinQuery) multi() (interface{}, error) {
	tx := q.prepare()

	objValue := makeSlice(reflect.TypeOf(q.ModelStructs.raw), 0)
	objArr := objValue.Interface()

	return pages(tx, q.search, objArr, true)
}
