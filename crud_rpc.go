package kitty

// RPC rpc call
type RPC interface {
	Call(model string, search *SearchCondition, action string) (interface{}, error)
}

//RPCCrud remote call
type RPCCrud struct {
	Model interface{}
	RPC   RPC
}

// Do 调用rpc远程执行
func (crud *RPCCrud) Do(search *SearchCondition, action string, c Context) (interface{}, error) {
	// 调rpc方法
	model := CreateModelStructs(crud.Model).Name()
	return crud.RPC.Call(model, search, action)
}
