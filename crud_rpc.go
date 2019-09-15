package kitty

import "errors"

// RPC rpc call
type RPC interface {
	Call(model string, search *SearchCondition, action string) (interface{}, error)
}

//RPCCrud 本地操作
type RPCCrud struct {
	Model string
	RPC   RPC
}

// Do 调用rpc远程执行
func (crud *RPCCrud) Do(search *SearchCondition, action string, c context) (interface{}, error) {
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

	// TODO 参数校验
	// 调rcp方法
	return crud.RPC.Call(crud.Model, search, action)
}
