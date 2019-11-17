package rpc

import (
	"context"
	"fmt"

	"github.com/jinzhu/gorm"

	"github.com/xmdas-link/kitty"
	kittyrpc "github.com/xmdas-link/kitty/rpc/proto/kittyrpc"
)

// SrvContext get something from context
type SrvContext interface {
	GetCtxInfo(context.Context, string) (interface{}, error)
}

//KittyRPCService ...
type KittyRPCService struct {
	DB     *gorm.DB
	Callbk kitty.SuccessCallback
	Ctx    SrvContext
	Params map[string]interface{}
}

type rpcContext struct {
	ctx    context.Context
	srvCtx SrvContext
}

func (c *rpcContext) CurrentUID() (string, error) {
	s, err := c.srvCtx.GetCtxInfo(c.ctx, "loginid")
	if err != nil {
		return "", err
	}
	return s.(string), nil
}
func (c *rpcContext) GetCtxInfo(s string) (interface{}, error) {
	if s == "ContextRPC" {
		return c.ctx, nil
	}
	return c.srvCtx.GetCtxInfo(c.ctx, s)
}

// Call rpc call handle
func (rpc *KittyRPCService) Call(ctx context.Context, req *kittyrpc.Request, rsp *kittyrpc.Response) error {

	var (
		err       error
		res       interface{}
		jsonbytes []byte
	)

	if !kitty.HasRegister(req.Model) {
		return fmt.Errorf("kittyrpc service: %s not exist", req.Model)
	}

	search := kitty.SearchCondition{}
	err = json.Unmarshal([]byte(req.Search), &search)
	if err != nil {
		return err
	}
	search.Params = rpc.Params
	if search.Params == nil {
		search.Params = make(map[string]interface{})
	}
	if rpc.DB == nil {
		obj, err := rpc.Ctx.GetCtxInfo(ctx, "ContextDB")
		if err == nil {
			rpc.DB = obj.(*gorm.DB)
		} else {
			return err
		}
	}
	rpcCtx := &rpcContext{
		ctx:    ctx,
		srvCtx: rpc.Ctx,
	}

	crud := &kitty.LocalCrud{
		Model:  req.Model,
		DB:     rpc.DB,
		Callbk: rpc.Callbk,
		RPC:    &KittyClientRPC{},
	}
	fmt.Printf("rpc %s call\n", req.Model)

	if res, err = crud.Do(&search, req.Action, rpcCtx); err == nil && res != nil {
		if jsonbytes, err = json.Marshal(res); err == nil {
			rsp.Msg = string(jsonbytes)
			return nil
		}
	}

	return err
}
