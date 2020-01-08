package kitty

import (
	"errors"
	"fmt"
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
	RPC    RPC
}

// Do 本地执行db操作
func (local *LocalCrud) Do(search *SearchCondition, action string, c Context) (interface{}, error) {
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

	var (
		res interface{}
		err error
		tx  *gorm.DB
	)
	if local.DB != nil {
		tx = local.DB.Begin()
	}

	defer func() {
		if r := recover(); r != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			log.Printf("Panic: %s, Action: %s==> %s\n", r, action, string(buf[:n]))
			err = errors.New("panic")
		}
		if tx != nil {
			if err == nil {
				tx.Commit()
			} else {
				tx.Rollback()
			}
		}
	}()

	// getter -> plugin -> crud -> setter -> callback
	if err = Getter(s, make(map[string]interface{}), tx, c); err != nil {
		if f, ok := s.FieldOk("KittyCode"); ok {
			return nil, fmt.Errorf("kittycode:%d,%s", f.Value().(int), err.Error())
		}
		return nil, err
	}
	if local.RPC != nil {
		if err = local.RPC.WebCall(s, search, c); err != nil {
			if f, ok := s.FieldOk("KittyCode"); ok {
				return nil, fmt.Errorf("kittycode:%d,%s", f.Value().(int), err.Error())
			}
			return nil, err
		}
	}

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
	case "RPC": // do nothing
		res = s.raw
	default:
		err = errors.New("unknown model action")
	}

	if err = Setter(s, make(map[string]interface{}), tx, c); err != nil {
		if f, ok := s.FieldOk("KittyCode"); ok {
			return nil, fmt.Errorf("kittycode:%d,%s", f.Value().(int), err.Error())
		}
		return nil, err
	}
	if local.Callbk != nil {
		if err = local.Callbk(s, tx); err != nil {
			if f, ok := s.FieldOk("KittyCode"); ok {
				return nil, fmt.Errorf("kittycode:%d,%s", f.Value().(int), err.Error())
			}
			return nil, err
		}
	}

	if err == nil {
		nameAs := make(map[string][]string)
		result := CrudResult{
			Code: 1,
			Ref:  time.Now().UnixNano() / 1e6,
		}
		if f, ok := s.FieldOk("KittyCode"); ok {
			result.Code = f.Value().(int)
		}
		if res != nil {
			result.Data = res
			s.nameAs(nameAs)
		} else if action == "R" {
			result.Code = 0
			result.Message = "007"
		}
		return &Result{
			result,
			nameAs,
			jsoniter.Config{}.Froze(),
		}, nil
	}

	if _, ok := err.(*mysql.MySQLError); ok {
		if kittyMode == debugCode {
			return nil, err
		}
		return nil, errors.New("0071")
	}
	if f, ok := s.FieldOk("KittyCode"); ok {
		return nil, fmt.Errorf("kittycode:%d,%s", f.Value().(int), err.Error())
	}
	return res, err
}
