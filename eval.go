package kitty

import (
	"reflect"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/jinzhu/gorm"
)

type qry interface {
	multi() (interface{}, error)
	one() (interface{}, error)
}
type update interface {
	create() (interface{}, error)
	update() error
}

func getter(s *Structs, param map[string]interface{}, db *gorm.DB, c context) error {
	expr := &expr{
		db:        db,
		s:         s,
		functions: make(map[string]govaluate.ExpressionFunction),
		params:    param,
		ctx:       c,
	}
	expr.params["s"] = s.raw
	expr.init()
	for _, f := range s.Fields() {
		if getter := GetSub(f.Tag("kitty"), "getter"); len(getter) > 0 {
			expr.f = f
			res, err := expr.eval(getter)
			if err != nil {
				return err
			}
			if res != nil {
				s.SetFieldValue(f, res)
			}
		}
	}
	return nil
}

func setter(s *Structs, param map[string]interface{}, db *gorm.DB, c context) error {
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
			tk := (&FormField{f}).TypeAndKind()
			if tk.TypeOfField.Kind() == reflect.Slice {
				rv := reflect.ValueOf(f.Value())
				len := rv.Len()
				for i := 0; i < len; i++ {
					rvdata := rv.Index(i)
					sdata := createModelStructs(rvdata.Interface())
					if err := setter(sdata, param, db, c); err != nil {
						return err
					}
				}
			} else if tk.TypeOfField.Kind() == reflect.Struct {
				sdata := createModelStructs(f.Value())
				if err := setter(sdata, param, db, c); err != nil {
					return err
				}
			}
		} else if setter := GetSub(k, "setter"); len(setter) > 0 {
			expr.f = f
			if strings.Contains(setter, ".") {
				a := strings.LastIndex(setter, "(")
				b := strings.Index(setter, ".")
				model := setter[a+1 : b]
				//res, err := queryObj(NewModelStruct(model), &SearchCondition{}, db, c)
				res, err := queryObj(s.createModelStructs(model), &SearchCondition{}, db, c)
				if err != nil {
					return err
				}
				expr.params[model] = res
			}
			res, err := expr.eval(setter)
			if err != nil {
				return err
			}
			if res != nil {
				s.SetFieldValue(f, res)
			}
		}
	}
	return nil
}

func evalJoin(s *Structs, kittys *kittys, search *SearchCondition, db *gorm.DB) (interface{}, error) {
	joinqry := &joinQuery{
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
	return execqry(joinqry, kittys.multiResult)
}

func evalSimpleQry(s *Structs, kittys *kittys, search *SearchCondition, db *gorm.DB) (interface{}, error) {
	Master := kittys.master()
	qry := &query{
		db:           db,
		search:       search,
		ModelStructs: kittys.result,
		queryString:  s.buildFormQuery(Master.TableName, Master.ModelName),
	}
	return execqry(qry, kittys.multiResult)
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
		//res, err = q.one()
		if res, err = q.one(); err == nil && res != nil {
			res = reflect.ValueOf(res).Interface()
		}
	}
	return res, err
}
