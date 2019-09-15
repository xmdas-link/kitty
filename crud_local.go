package kitty

import (
	"errors"

	"github.com/jinzhu/gorm"
	jsoniter "github.com/json-iterator/go"
)

//LocalCrud 本地操作
type LocalCrud struct {
	Model string
	DB    *gorm.DB
}

// Do 本地执行db操作
func (crud *LocalCrud) Do(search *SearchCondition, action string, c context) (interface{}, error) {
	if err := search.CheckParamValid(crud.Model); err != nil {
		return nil, err
	}

	s := NewModelStruct(crud.Model)
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

	switch action {
	case "C":
		res, err = createObj(s, search, crud.DB, c)
	case "R":
		res, err = queryObj(s, search, crud.DB, c)
	case "U":
		err = updateObj(s, search, crud.DB, c)
	default:
		return nil, errors.New("unknown model action")
	}
	if res != nil {
		result := CrudResult{
			Code: 1,
			Data: res,
		}
		NameAs := make(map[string][]string)
		s.nameAs(NameAs)
		if search.Page != nil {
			result.Page = search.Page
			result.Count = new(int)
			*result.Count = search.ReturnCount
		}
		return &Result{
			result,
			NameAs,
			jsoniter.Config{}.Froze(),
		}, nil
	}
	return nil, err
}
