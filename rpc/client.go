package rpc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
	"github.com/json-iterator/go"
	"github.com/micro/go-micro/client"

	"github.com/xmdas-link/kitty"
	kittyrpc "github.com/xmdas-link/kitty/rpc/proto/kittyrpc"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// KittyClientRPC kitty for rpc
type KittyClientRPC struct {
	CliService kittyrpc.KittyrpcService
	Model      interface{}
	Callbk     kitty.SuccessCallback
}

// Call 调用rpc服务端
func (rpc *KittyClientRPC) Call(search *kitty.SearchCondition, action string, c kitty.Context) (interface{}, error) {
	if action == "RPC" {
		return rpc.localCall(search, c)
	}
	res, err := json.Marshal(search)
	if err != nil {
		return nil, err
	}
	model := kitty.CreateModelStructs(rpc.Model).Name()
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

/*
type PageDevice struct {
	SchoolRsp *deviceProto.GetResponse  `json:"-" kitty:"call:schoolClient.GetSchool"` //rpc执行结果
	DeviceRsp *deviceProto.PageResponse `json:"-" kitty:"call:deviceClient.Page"`
	UsersRsp  *userProto.UsersResponse  `json:"-" kitty:"call:userClient.GetUsers"`

	GetRequest   *schoolProto.GetRequest  `json:"-" kitty:"protocol:schoolClient.GetSchool"`
	PageRequest  *deviceProto.PageRequest `json:"-" kitty:"protocol:deviceClient.Page"` //rpc请求参数的声明
	UsersRequest *userProto.UsersRequest  `json:"-" kitty:"protocol:userClient.GetUsers"`

	Page          *deviceProto.Page `json:"-" kitty:"param:PageRequest.Page"` //rpc请求参数绑定
	SchoolId      *uint             `json:"-" kitty:"param:PageRequest.SchoolId"`
	SchoolStdCode *string           `json:"-" kitty:"param:PageRequest.SchoolStdCode;getter:f('schoolRsp.School.SchoolStdCode')"`
	UserNames     []string          `json:"-" kitty:"param:UserRequest.UserNames;getter:f('DeviceRsp.DeviceList[*].EnrollNo')"`

	User     userProto.User         `json:"-"`
	List     []*deviceData          `kitty:"setter:f('DeviceRsp.DeviceList')"` //结果
	UsersMap map[string]*deviceUser `kitty:"key:user.name;setter:f('UsersRsp.User')"`
}
*/

func (rpc *KittyClientRPC) localCall(search *kitty.SearchCondition, c kitty.Context) (interface{}, error) {

	defer func() {
		if r := recover(); r != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			fmt.Printf("Action: %s==> %s\n", "localRPC", string(buf[:n]))
		}
	}()

	s := kitty.CreateModelStructs(rpc.Model)
	if err := s.ParseFormValues(search.FormValues); err != nil {
		return nil, err
	}
	var (
		ctx        = c.GetCtx()
		rpcClients = search.Params
	)

	type localRPC struct {
		name        string
		client      *kitty.Structs
		method      string
		methodField string
		param       *kitty.Structs
		result      *structs.Field
	}

	if err := vd.Validate(s.Raw()); err != nil {
		return nil, err
	}
	if err := kitty.Getter(s, make(map[string]interface{}), nil, c); err != nil {
		return nil, err
	}

	rpcs := make([]*localRPC, 0)
	var getrpc = func(client string, method string) *localRPC {
		for _, v := range rpcs {
			if v.name == client && v.method == method {
				return v
			}
		}
		return nil
	}

	for _, f := range s.Fields() {
		if k := f.Tag("kitty"); len(k) > 0 {
			tk := kitty.TypeKind(f)
			if call := kitty.GetSub(k, "call"); len(call) > 0 {
				// UsersRsp  *userProto.UsersResponse  `json:"-" kitty:"call:userClient.GetUsers"`
				v := strings.Split(call, ".")
				rpcs = append(rpcs, &localRPC{
					name:   v[0],
					client: kitty.CreateModelStructs(rpcClients[v[0]]),
					method: v[1],
					result: f,
				})
			}
			if protocol := kitty.GetSub(k, "protocol"); len(protocol) > 0 {
				// GetRequest   *schoolProto.GetRequest  `json:"-" kitty:"protocol:schoolClient.GetSchool"`
				methodStrs := tk.Create()
				if err := f.Set(methodStrs.Raw()); err != nil {
					return nil, err
				}
				v := strings.Split(protocol, ".")
				rpc := getrpc(v[0], v[1])
				rpc.methodField = f.Name() // PageRequest DeviceRequest KittyPersonRequest
				rpc.param = methodStrs
			}
		}
	}

	for _, rpc := range rpcs {
		//	SchoolId      *uint             `json:"-" kitty:"param:PageRequest.SchoolId;runtime:set(0)"`
		paramformat := fmt.Sprintf("%s.", rpc.methodField) // PageRequest DeviceRequest
		paramStrs := rpc.param
		isKittyRequest := false
		for _, f := range s.Fields() {
			if k := f.Tag("kitty"); len(k) > 0 && strings.Contains(k, "param:") {
				if param := kitty.GetSub(k, "param"); len(param) > 0 {
					v := strings.Split(param, ".")
					if v[1] == "Model" && v[0] == rpc.methodField { // like KittyPersonRequest.Model; for kitty rpc request param->model
						paramformat = fmt.Sprintf("%s.", f.Name()) // param:ModelPerson.xxx
						tk := kitty.TypeKind(f)
						paramStrs = tk.Create()
						if err := f.Set(paramStrs.Raw()); err != nil {
							return nil, err
						}
						rpc.param.Field("Model").Set(tk.ModelName)
						isKittyRequest = true
					} else if strings.Contains(param, paramformat) {
						if runtime := kitty.GetSub(k, "runtime"); len(runtime) > 0 {
							if err := kitty.Eval(s, nil, f, runtime); err != nil {
								return nil, err
							}
						}
						ff := paramStrs.Field(v[1])
						if err := paramStrs.SetFieldValue(ff, f.Value()); err != nil {
							return nil, err
						}
					}
				}
			}
			if isKittyRequest {
				// format search condition.
				form := make(map[string][]string)
				for _, f := range paramStrs.Fields() {
					if k := f.Tag("kitty"); len(k) > 0 && strings.Contains(k, "param:") && !strings.Contains(k, "-;param") {
						x := ""
						rv := kitty.DereferenceValue(reflect.ValueOf(f.Value()))
						if rv.Kind() >= reflect.Bool && rv.Kind() <= reflect.Float64 {
							x = fmt.Sprintf("%v", rv)
						} else if rv.Kind() == reflect.String {
							x = rv.Interface().(string)
						}
						if len(x) == 0 {
							continue
						}
						name := strcase.ToSnake(f.Name())
						if v := form[name]; v == nil {
							form[name] = []string{}
						}
						v := form[name]
						v = append(v, x)
						form[name] = v
					}
				}
				search := &kitty.SearchCondition{
					FormValues: form,
				}
				res, err := json.Marshal(search)
				if err != nil {
					return nil, err
				}
				rpc.param.Field("Search").Set(string(res))
			}
		}
		var a = func(options *client.CallOptions) {
		}
		values := rpc.client.CallMethod(rpc.method, reflect.ValueOf(ctx), reflect.ValueOf(rpc.param.Raw()), reflect.ValueOf(a))

		if kitty.DereferenceValue(values[1]).Kind() != reflect.Invalid {
			return nil, values[1].Interface().(error)
		}

		rspValue := values[0].Interface()
		if rpc.method == "Call" {
			rpcrsp := rspValue.(*kittyrpc.Response)
			if len(rpcrsp.Msg) > 0 {
				res := &kitty.CrudResult{}
				if err := json.Unmarshal([]byte(rpcrsp.Msg), res); err != nil {
					return nil, err
				}
				if res.Code != 1 {
					return nil, errors.New(res.Message)
				}
				rspValue = res.Data
			}
		}

		if err := rpc.result.Set(rspValue); err != nil {
			return nil, err
		}

	}
	if err := kitty.Setter(s, make(map[string]interface{}), nil, c); err != nil {
		return nil, err
	}

	if rpc.Callbk != nil {
		if err := rpc.Callbk(s, nil); err != nil {
			return nil, err
		}
	}

	return s.Raw(), nil
}
