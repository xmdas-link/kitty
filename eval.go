package kitty

import (
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"

	"github.com/Knetic/govaluate"
	"github.com/jinzhu/gorm"
)

type qry interface {
	prepare() *gorm.DB
	multi() (interface{}, error)
	one() (interface{}, error)
}
type update interface {
	create() (interface{}, error)
	update() error
}

func getter(s *Structs, param map[string]interface{}, db *gorm.DB, c Context) error {
	expr := &expr{
		db:        db,
		s:         s,
		functions: make(map[string]govaluate.ExpressionFunction),
		params:    param,
		ctx:       c,
	}
	expr.init()
	for _, f := range s.Fields() {
		if getter := GetSub(f.Tag("kitty"), "getter"); len(getter) > 0 {
			expr.f = f
			if err := expr.eval(getter); err != nil {
				return err
			}
		}
	}
	return nil
}

func setter(s *Structs, param map[string]interface{}, db *gorm.DB, c Context) error {
	expr := &expr{
		db:        db,
		s:         s,
		functions: make(map[string]govaluate.ExpressionFunction),
		params:    param,
		ctx:       c,
	}
	expr.init()
	for _, f := range s.Fields() {
		k := f.Tag("kitty")
		if strings.Contains(k, "bindresult") {
			tk := TypeKind(f)
			if tk.KindOfField == reflect.Slice {
				rv := reflect.ValueOf(f.Value())
				len := rv.Len()
				for i := 0; i < len; i++ {
					rvdata := rv.Index(i)
					sdata := CreateModelStructs(rvdata.Interface())
					if err := setter(sdata, param, db, c); err != nil {
						return err
					}
				}
			} else if tk.KindOfField == reflect.Struct {
				sdata := CreateModelStructs(f.Value())
				if err := setter(sdata, param, db, c); err != nil {
					return err
				}
			}
		} else if setter := GetSub(k, "setter"); len(setter) > 0 {
			expr.f = f
			if err := expr.eval(setter); err != nil {
				return err
			}
		}
	}
	return nil
}

func evalJoin(s *Structs, kittys *kittys, search *SearchCondition, db *gorm.DB) qry {
	kittys.prepare()
	return &joinQuery{
		db:           db,
		search:       search,
		ModelStructs: kittys.result,
		TableName:    kittys.master().TableName,
		Selects:      kittys.selects(),
		Joins:        kittys.joins(),
		Where:        kittys.where(),
		GroupBy:      kittys.groupby(),
		Having:       kittys.having(),
	}
	//return execqry(joinqry, kittys.multiResult)
}

func evalSimpleQry(s *Structs, kittys *kittys, search *SearchCondition, db *gorm.DB) qry {
	modelName := strcase.ToSnake(kittys.master().ModelName)
	var qryformats []*fieldQryFormat
	for _, v := range s.buildAllParamQuery() {
		if v.model == modelName {
			qryformats = append(qryformats, v)
		}
	}

	q := &query{
		db:           db,
		search:       search,
		ModelStructs: kittys.result,
		queryString:  qryformats,
	}
	if len(kittys.binds) > 0 && kittys.binds[0] != nil {
		q.fieldselect = kittys.binds[0].BindModelField
	}
	return q
	//	return execqry(qry, kittys.multiResult)
}

func execqry(q qry, multi bool) (interface{}, error) {
	var (
		res interface{}
		err error
	)
	if multi {
		if res, err = q.multi(); err == nil && res != nil {
			res = reflect.ValueOf(res).Elem().Interface()
		}
	} else {
		if res, err = q.one(); err != nil {
			return nil, err
		}
	}
	return res, err
}
