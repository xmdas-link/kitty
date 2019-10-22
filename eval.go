package kitty

import (
	"reflect"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/jinzhu/gorm"
)

type qry interface {
	query() *gorm.DB
	multi() (interface{}, error)
	one() (interface{}, error)
}
type update interface {
	create() (interface{}, error)
	update() error
}

// Getter before execute
func Getter(s *Structs, param map[string]interface{}, db *gorm.DB, c Context) error {
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
		if getter := GetSub(k, "getter"); len(getter) > 0 {
			expr.f = f
			if err := expr.eval(getter); err != nil {
				return err
			}
		}
	}
	return nil
}

// Setter after execute successful
func Setter(s *Structs, param map[string]interface{}, db *gorm.DB, c Context) error {
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
		if setter := GetSub(k, "setter"); len(setter) > 0 {
			expr.f = f
			if err := expr.eval(setter); err != nil {
				return err
			}
		}
		if strings.Contains(k, "bindresult") && !f.IsZero() {
			tk := TypeKind(f)
			if tk.KindOfField == reflect.Slice {
				rv := reflect.ValueOf(f.Value())
				len := rv.Len()
				for i := 0; i < len; i++ {
					rvdata := rv.Index(i)
					sdata := CreateModelStructs(rvdata.Interface())
					if err := Setter(sdata, param, db, c); err != nil {
						return err
					}
				}
			} else if tk.KindOfField == reflect.Struct {
				sdata := CreateModelStructs(f.Value())
				if err := Setter(sdata, param, db, c); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func evalJoin(s *Structs, kittys *kittys, search *SearchCondition, db *gorm.DB) qry {
	kittys.prepare()
	return &joinQuery{
		db:           db,
		kittys:       kittys,
		search:       search,
		ModelStructs: kittys.result,
		TableName:    kittys.master().TableName,
		Selects:      kittys.selects(),
		Joins:        kittys.joins(),
		Where:        kittys.where(),
		GroupBy:      kittys.groupby(),
		Having:       kittys.having(),
		order:        kittys.order(),
	}
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
