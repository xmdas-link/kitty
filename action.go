package kitty

import (
	"errors"

	"github.com/json-iterator/go"

	"github.com/jinzhu/gorm"
)

// ActionCRUD ...
func ActionCRUD(db *gorm.DB, model string, search *SearchCondition, ac string, multi bool) (interface{}, error) {

	s := NewModelStruct(model)
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

	switch ac {
	case "C":
		res, err = createObj(s, search, db)
	case "R":
		res, err = queryObj(s, search, db)
	case "U":
		err = updateObj(s, search, db)
	default:
		return nil, errors.New("unknown model action")
	}

	if res != nil {
		cfg := jsoniter.Config{}.Froze()
		cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, []string{}, ""})
		nameas := make(map[string][]string)
		s.nameAs(nameas)
		for k, v := range nameas {
			cfg.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}, v, k})
		}
		jsoniter.RegisterTypeEncoder("time.Time", &timeAsString{})
		msg, err := cfg.Marshal(res)
		return string(msg), err
	}
	return nil, err
}
