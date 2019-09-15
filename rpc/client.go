package rpc

import (
	"context"
	"encoding/json"

	"github.com/xmdas-link/kitty"
	kittyrpc "github.com/xmdas-link/kitty/rpc/proto/kittyrpc"
)

// KittyClientRPC kitty for rpc
type KittyClientRPC struct {
	CliService kittyrpc.KittyrpcService
}

// Call 调用rpc服务端
func (rpc *KittyClientRPC) Call(model string, search *kitty.SearchCondition, action string) (interface{}, error) {
	res, err := json.Marshal(search)
	if err != nil {
		return nil, err
	}
	req := kittyrpc.Request{
		Model:  model,
		Action: action,
		Search: string(res),
	}
	rsp := &kittyrpc.Response{}

	if rsp, err = rpc.CliService.Call(context.TODO(), &req); err == nil {
		if len(rsp.Msg) > 0 {
			res := &kitty.CrudResult{}
			err = json.Unmarshal([]byte(rsp.Msg), res)
			return res, err
		}
		return nil, nil
	}

	return nil, err
}
