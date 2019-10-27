package kitty

import (
	"errors"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	jsoniter "github.com/json-iterator/go"
)

// RPC rpc call
type RPC interface {
	Call(*SearchCondition, string, Context) (interface{}, error)
}

//RPCCrud remote call
type RPCCrud struct {
	RPC RPC
}

// Do 调用rpc远程执行
func (crud *RPCCrud) Do(search *SearchCondition, action string, c Context) (interface{}, error) {
	// 调rpc方法
	//model := CreateModelStructs(crud.Model).Name()
	res, err := crud.RPC.Call(search, action, c)
	if res != nil && action == "RPC" {
		result := CrudResult{
			Code: 1,
			Ref:  time.Now().UnixNano() / 1e6,
			Data: res,
		}
		nameAs := make(map[string][]string)
		s := CreateModelStructs(res)
		s.nameAs(nameAs)
		return &Result{
			result,
			nameAs,
			jsoniter.Config{}.Froze(),
		}, nil
	}
	log.Println(err)
	if _, ok := err.(*mysql.MySQLError); ok {
		if kittyMode == debugCode {
			return nil, err
		}
		return nil, errors.New("数据库执行错误")
	}
	return res, err
}
