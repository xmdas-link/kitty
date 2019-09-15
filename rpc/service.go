package rpc

import (
	"context"
	"encoding/json"

	"github.com/jinzhu/gorm"

	"github.com/xmdas-link/kitty"
	kittyrpc "github.com/xmdas-link/kitty/rpc/proto/kittyrpc"
)

//KittyRPCService ...
type KittyRPCService struct {
	DB *gorm.DB
}

// Handler rpc call handle
func (rpc *KittyRPCService) Call(ctx context.Context, req *kittyrpc.Request, rsp *kittyrpc.Response) error {

	var (
		err       error
		res       interface{}
		jsonbytes []byte
	)

	search := kitty.SearchCondition{}
	err = json.Unmarshal([]byte(req.Search), &search)
	if err != nil {
		return err
	}

	crud := &kitty.LocalCrud{
		Model: req.Model,
		DB:    rpc.DB,
	}

	if res, err = crud.Do(&search, req.Action, nil); err == nil && res != nil {
		if jsonbytes, err = json.Marshal(res); err == nil {
			rsp.Msg = string(jsonbytes)
			return nil
		}
	}

	return err
}
