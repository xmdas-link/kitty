package kitty

import (
	"errors"
	"log"
	"runtime"
	"time"

	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	jsoniter "github.com/json-iterator/go"
)

//LocalCrud 本地操作
type LocalCrud struct {
	Model  string // RPC 最终会调用此，所以只能用model作为参数。
	Strs   *Structs
	DB     *gorm.DB
	Callbk SuccessCallback
}

// Validate ...
func (local *LocalCrud) Validate(search *SearchCondition, c Context) (*Structs, error) {
	s := local.Strs
	if s == nil {
		s = CreateModel(local.Model)
	}
	if s == nil {
		return nil, errors.New("error in create model")
	}
	if err := s.ParseFormValues(search.FormValues); err != nil {
		return nil, err
	}
	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}
	if err := Getter(s, make(map[string]interface{}), local.DB, c); err != nil {
		return nil, err
	}
	return s, nil
}

// Do 本地执行db操作
func (local *LocalCrud) Do(search *SearchCondition, action string, c Context) (interface{}, error) {

	s, err := local.Validate(search, c)
	if err != nil {
		return nil, err
	}

	return local.Action(s, search, action, c)
}

// Action ...
func (local *LocalCrud) Action(s *Structs, search *SearchCondition, action string, c Context) (interface{}, error) {
	var (
		res interface{}
		err error
	)
	tx := local.DB.Begin()

	defer func() {
		if r := recover(); r != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			log.Printf("Panic: %s, Action: %s==> %s\n", r, action, string(buf[:n]))
			tx.Rollback()
		}
	}()

	crud := newcrud(&config{
		strs:   s,
		search: search,
		db:     tx,
		ctx:    c,
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
		err = Setter(s, make(map[string]interface{}), tx.New(), c)
		if local.Callbk != nil && err == nil {
			err = local.Callbk(s, tx.New())
		}
	}
	if err == nil {
		err = tx.Commit().Error

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
			result.Message = "发生未知错误"
		}
		return &Result{
			result,
			nameAs,
			jsoniter.Config{}.Froze(),
		}, nil
	}
	tx.Rollback()
	if _, ok := err.(*mysql.MySQLError); ok {
		if kittyMode == debugCode {
			return nil, err
		}
		return nil, errors.New("数据库执行错误")
	}
	return res, err
}
