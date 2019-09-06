package kitty

import (
	"fmt"
	"time"

	tagexpr "github.com/bytedance/go-tagexpr"
	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/jinzhu/gorm"
)

func init() {
	vd.RegFunc("time", func(args ...interface{}) bool {
		if len(args) != 1 {
			return false
		}
		s, ok := args[0].(string)
		if !ok {
			return false
		}
		_, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
		if err != nil {
			return false
		}
		return true
	}, true)
}

func queryObj(s *Structs, search *SearchCondition, db *gorm.DB) (interface{}, error) {
	getter(s, make(map[string]interface{}), db)

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}

	kittys := &kittys{
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(); err != nil {
		return nil, err
	}
	var (
		res interface{}
		err error
	)

	if len(kittys.kittys) > 1 {
		res, err = evalJoin(s, kittys, search, db)
	} else {
		res, err = evalSimpleQry(s, kittys, search, db)
	}
	if err != nil || res == nil {
		return nil, err
	}
	if err = s.Field("Data").Set(res); err != nil {
		return nil, err
	}

	params := make(map[string]interface{})
	params["ms"] = s
	params["kittys"] = kittys
	if err = setter(s, params, db); err != nil {
		return nil, err
	}
	return s.raw, nil
}

// CreateObj ...
func createObj(s *Structs, search *SearchCondition, db *gorm.DB) (interface{}, error) {

	getter(s, make(map[string]interface{}), db)

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}
	kittys := &kittys{
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(); err != nil {
		return nil, err
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("create error. something happen...")
			tx.Rollback()
		}
	}()

	qry := &simpleQuery{
		db:           tx,
		ModelStructs: s,
		search:       search,
		Result:       kittys.master().structs,
	}
	for _, v := range kittys.kittys {
		if !v.Master {
			qry.Next = append(qry.Next, &simpleQuery{
				db:           tx,
				ModelStructs: s,
				search:       &SearchCondition{},
				Result:       v.structs,
			})
		}
	}
	res, err := qry.create()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return res, tx.Commit().Error
}

func updateObj(s *Structs, search *SearchCondition, db *gorm.DB) error {

	getter(s, make(map[string]interface{}), db)

	vm := tagexpr.New("te")
	r := vm.MustRun(s.raw)
	if !r.Eval("ID").(bool) {
		return fmt.Errorf(r.Eval("ID@msg").(string))
	}

	if err := vd.Validate(s.raw); err != nil {
		return err
	}

	kittys := &kittys{
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(); err != nil {
		return err
	}
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("update error. something happen...")
			tx.Rollback()
		}
	}()

	qry := &simpleQuery{
		db:           tx,
		ModelStructs: s,
		search:       search,
		Result:       kittys.master().structs,
	}
	for _, v := range kittys.kittys {
		if !v.Master {
			qry.Next = append(qry.Next, &simpleQuery{
				db:           tx,
				ModelStructs: s,
				search:       &SearchCondition{},
				Result:       v.structs,
			})
		}
	}

	if err := qry.update(); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}
