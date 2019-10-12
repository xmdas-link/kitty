package kitty

import (
	"errors"
	"time"

	"github.com/jinzhu/gorm"
	jsoniter "github.com/json-iterator/go"
)

//LocalCrud 本地操作
type LocalCrud struct {
	Model  string // RPC 最终会调用此，所以只能用model作为参数。
	DB     *gorm.DB
	Callbk SuccessCallback
}

// Do 本地执行db操作
func (local *LocalCrud) Do(search *SearchCondition, action string, c Context) (interface{}, error) {
	//	if err := search.CheckParamValid(crud.Model); err != nil {
	//		return nil, err
	//	}

	s := CreateModel(local.Model)
	if s == nil {
		return nil, errors.New("error in create model")
	}
	if err := s.ParseFormValues(search.FormValues); err != nil {
		return nil, err
	}

	var (
		res interface{}
		err error
	)
	crud := newcrud(&config{
		strs:   s,
		search: search,
		db:     local.DB,
		ctx:    c,
		callbk: local.Callbk,
	})

	switch action {
	case "C":
		res, err = crud.createObj()
	case "R":
		res, err = crud.queryObj()
	case "U":
		res, err = crud.updateObj()
	default:
		return nil, errors.New("unknown model action")
	}
	if err == nil {
		nameAs := make(map[string][]string)
		result := CrudResult{
			Code: 1,
			Ref:  time.Now().UnixNano() / 1e6,
		}
		if res != nil {
			result.Data = res
			s.nameAs(nameAs)
		} else if action == "R" {
			result.Code = 0
			result.Message = "没有记录"
		}
		return &Result{
			result,
			nameAs,
			jsoniter.Config{}.Froze(),
		}, nil
	}
	return nil, err
}
