package kitty

// RPC rpc call
type RPC interface {
	Call(*SearchCondition, string, Context) (interface{}, error)
	WebCall(*Structs, *SearchCondition, Context) error
}

//RPCCrud remote call
type RPCCrud struct {
	RPC RPC
}

// Do 调用rpc远程执行
func (crud *RPCCrud) Do(search *SearchCondition, action string, c Context) (interface{}, error) {
	// 调rpc方法
	return crud.RPC.Call(search, action, c)
}
