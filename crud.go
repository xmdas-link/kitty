package kitty

import (
	"fmt"

	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/jinzhu/gorm"
)

//CRUDInterface ...
type CRUDInterface interface {
	Do(*SearchCondition, string, Context) (interface{}, error)
}

// SuccessCallback 执行成功后，回调。返回error后，回滚事务
type SuccessCallback func(*Structs, *gorm.DB) error

// CRUD配置
type config struct {
	strs   *Structs         //模型结构
	search *SearchCondition //查询条件
	db     *gorm.DB         //db
	ctx    Context          //上下文
	callbk SuccessCallback  //成功回调
}

type crud struct {
	*config
}

func newcrud(conf *config) *crud {
	return &crud{conf}
}

func (crud *crud) queryObj() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
		callbk = crud.callbk
	)
	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

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
	if err = setter(s, params, db, c); err != nil {
		return nil, err
	}

	if callbk != nil {
		if err = callbk(s, db); err != nil {
			return nil, err
		}
	}

	return s.raw, nil
}

// CreateObj ...
func (crud *crud) createObj() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
		callbk = crud.callbk
	)
	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

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

	if kittyMode == releaseCode {
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				fmt.Println("create error. something happen...")
			}
		}()
	}

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
	for _, v := range kittys.kittys {
		f := s.Field(v.FieldName)
		f.Set(v.structs.raw)
	}

	if f, ok := s.FieldOk("Data"); ok {
		if err := f.Set(res); err != nil {
			return nil, err
		}
	}

	params := make(map[string]interface{})
	params["ms"] = s
	params["kittys"] = kittys
	if err = setter(s, params, db, c); err != nil {
		return nil, err
	}
	if callbk != nil {
		if err = callbk(s, db); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return s.raw, tx.Commit().Error
}

func (crud *crud) updateObj() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
		callbk = crud.callbk
	)

	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

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
	if kittyMode == releaseCode {
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				fmt.Println("update error. something happen...")
			}
		}()
	}

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
		return nil, err
	}
	params := make(map[string]interface{})
	params["ms"] = s
	params["kittys"] = kittys
	if err := setter(s, params, db, c); err != nil {
		return nil, err
	}

	if callbk != nil {
		if err := callbk(s, db); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return s.raw, tx.Commit().Error
}

//
func queryObj(s *Structs, search *SearchCondition, db *gorm.DB, c Context) (interface{}, error) {
	crud := newcrud(&config{
		strs:   s,
		search: search,
		db:     db,
		ctx:    c,
	})
	return crud.queryObj()
}
