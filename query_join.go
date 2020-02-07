package kitty

import (
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
)

// JoinQuery ... 联表查询
type joinQuery struct {
	db           *gorm.DB
	kittys       *kittys
	search       *SearchCondition
	ModelStructs *Structs
	TableName    string
	Selects      *fieldQryFormat
	Joins        []*fieldQryFormat
	Where        []*fieldQryFormat
	GroupBy      []string
	Having       []*fieldQryFormat
	order        []*fieldQryFormat
}

func (q *joinQuery) prepare() *gorm.DB {
	tx := q.db.Table(q.TableName)
	for _, v := range q.Joins {
		tx = tx.Joins(v.operator, v.value...)
	}
	for _, v := range q.Where {
		tx = tx.Where(v.whereExpr(), v.values()...)
	}
	if len(q.GroupBy) > 0 {
		tx = tx.Group(strings.Join(q.GroupBy, ", "))
	}
	for _, v := range q.Having {
		tx = tx.Having(v.whereExpr(), v.value...)
	}
	return tx
}

func (q *joinQuery) hasGroupHaving() bool {
	return len(q.GroupBy) > 0 || len(q.Having) > 0
}

func (q *joinQuery) end(tx *gorm.DB) *gorm.DB {
	tx = tx.Select(q.Selects.bindfield, q.Selects.value...)
	for _, v := range q.order {
		tx = tx.Order(v.orderExpr())
	}
	return tx
}
func (q *joinQuery) query() *gorm.DB {
	tx := q.prepare()
	return q.end(tx)
}

func (q *joinQuery) one() (interface{}, error) {
	tx := q.prepare()
	tx = q.end(tx)

	if len(q.kittys.kittys) > 1 {
		rows, err := tx.Rows()
		if err != nil {
			return nil, err
		}
		if !rows.Next() {
			return nil, nil
		}
		defer func() {
			rows.Close()
		}()
		if err = tx.ScanRows(rows, q.ModelStructs.raw); err != nil {
			return nil, err
		}
	} else {
		if tx.First(q.ModelStructs.raw).RecordNotFound() {
			return nil, nil
		}
	}
	q.search.ReturnCount = 1
	return q.ModelStructs.raw, nil
}

func (q *joinQuery) multi() (interface{}, error) {
	tx := q.prepare()

	objValue := makeSlice(reflect.TypeOf(q.ModelStructs.raw), 0)
	result := objValue.Interface()

	scan := true

	if len(q.kittys.kittys) == 1 && q.kittys.master().ModelName == q.kittys.result.Name() {
		scan = false
	}

	if q.search.Page != nil {
		total := 0
		if q.hasGroupHaving() {
			tx.New().Raw("select count(1) from (?) tmp", tx.Select(q.kittys.selectForCount()).QueryExpr()).Count(&total)
		} else {
			tx.Count(&total)
		}
		//if scan {
		//	tx.Select(q.Selects.bindfield, q.Selects.value...).Count(&total)
		//} else {
		//	tx = tx.Count(&total)
		//}
		// 页数
		pageInfo := MakePage(q.search.Page.Page, q.search.Page.Limit, uint32(total))
		q.search.Page.Limit = pageInfo.Limit
		q.search.Page.Page = pageInfo.Page
		q.search.Page.Total = uint32(total)
		q.search.Page.PageMax = pageInfo.PageMax

		tx = tx.Offset(pageInfo.GetOffset()).Limit(pageInfo.Limit)
	}

	tx = q.end(tx)
	if scan {
		tx = tx.Scan(result)
	} else {
		tx = tx.Find(result)
	}

	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RecordNotFound() {
		return nil, nil
	}

	q.search.ReturnCount = int(tx.RowsAffected)

	return result, nil
	//return pages(tx, q.search, objArr, scan)

}
