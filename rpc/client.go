package rpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"strings"

	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/fatih/structs"
	"github.com/iancoleman/strcase"
	jsoniter "github.com/json-iterator/go"
	"github.com/micro/go-micro/client"

	"github.com/xmdas-link/kitty"
	kittyrpc "github.com/xmdas-link/kitty/rpc/proto/kittyrpc"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// KittyClientRPC kitty for rpc
type KittyClientRPC struct {
	CliService kittyrpc.KittyrpcService
	Model      interface{}
	modelname  string
	Callbk     kitty.SuccessCallback
}

type filterFieldsExtension struct {
	jsoniter.DummyExtension
}

func (extension *filterFieldsExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		if jsonTag := binding.Field.Tag().Get("json"); len(jsonTag) > 0 {
			if jsonTag != "omitempty" {
				continue
			}
		}
		binding.ToNames = []string{strcase.ToSnake(binding.Field.Name())}
		binding.FromNames = []string{strcase.ToSnake(binding.Field.Name())}
	}
}

func init() {
	jsoniter.RegisterExtension(&filterFieldsExtension{jsoniter.DummyExtension{}})
	jsoniter.RegisterTypeDecoder("time.Time", &kitty.TimeAsString{})
}

// Call 调用rpc服务端
func (rpc *KittyClientRPC) Call(search *kitty.SearchCondition, action string, c kitty.Context) (interface{}, error) {
	if action == "RPC" {
		s := kitty.CreateModelStructs(rpc.Model).New()
		if err := s.ParseFormValues(search.FormValues); err != nil {
			return nil, err
		}
		if err := vd.Validate(s.Raw()); err != nil {
			return nil, err
		}

		if err := kitty.Getter(s, search.Params, nil, c); err != nil {
			return nil, err
		}

		err := rpc.localCall(s, search, c)
		if err != nil {
			return nil, err
		}

		if err := kitty.Setter(s, search.Params, nil, c); err != nil {
			return nil, err
		}
		if rpc.Callbk != nil {
			if err := rpc.Callbk(s, nil); err != nil {
				return nil, err
			}
		}
		return s.Raw(), nil
	}

	res, err := json.Marshal(search)
	if err != nil {
		return nil, err
	}
	if len(rpc.modelname) == 0 {
		rpc.modelname = kitty.CreateModelStructs(rpc.Model).Name()
	}
	req := kittyrpc.Request{
		Model:  rpc.modelname,
		Action: action,
		Search: string(res),
	}
	rsp := &kittyrpc.Response{}

	ctx, _ := c.GetCtxInfo("ContextRPC")

	if rsp, err = rpc.CliService.Call(ctx.(context.Context), &req); err == nil {
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

type localRPC struct {
	name        string
	client      *kitty.Structs
	method      string
	methodField string
	param       *kitty.Structs
	result      *structs.Field
}

func (rpc *KittyClientRPC) localCall(s *kitty.Structs, search *kitty.SearchCondition, c kitty.Context) error {

	defer func() {
		if r := recover(); r != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			log.Printf("Panic %s, Action: %s==> %s\n", r, "localRPC", string(buf[:n]))
		}
	}()
	var (
		rpcParams = search.Params
	)
	ctx, _ := c.GetCtxInfo("ContextRPC")

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
					client: kitty.CreateModelStructs(rpcParams[v[0]]),
					method: v[1],
					result: f,
				})
			}
			if protocol := kitty.GetSub(k, "protocol"); len(protocol) > 0 {
				// GetRequest   *schoolProto.GetRequest  `json:"-" kitty:"protocol:schoolClient.GetSchool"`
				methodStrs := tk.Create()
				if err := f.Set(methodStrs.Raw()); err != nil {
					return err
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
							return err
						}
						rpc.param.Field("Model").Set(tk.ModelName)
						isKittyRequest = true
					} else if strings.Contains(param, paramformat) {
						if runtime := kitty.GetSub(k, "runtime"); len(runtime) > 0 {
							if err := kitty.Eval(s, nil, f, runtime); err != nil {
								return err
							}
						}
						if ff, ok := paramStrs.FieldOk(v[1]); ok {
							if err := paramStrs.SetFieldValue(ff, f.Value()); err != nil {
								return err
							}
						} else {
							return fmt.Errorf("%s field %s not exist", v[0], v[1])
						}

					}
				}
			}
		}
		if isKittyRequest {
			// format search condition.
			form := make(map[string][]string)
			for _, f := range paramStrs.Fields() {
				if k := f.Tag("kitty"); len(k) > 0 && strings.Contains(k, "param") && !strings.Contains(k, "-;param") {
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
				return err
			}
			rpc.param.Field("Search").Set(string(res))
		}

		var a = func(options *client.CallOptions) {}
		values := rpc.client.CallMethod(rpc.method, reflect.ValueOf(ctx), reflect.ValueOf(rpc.param.Raw()), reflect.ValueOf(a))

		if kitty.DereferenceValue(values[1]).Kind() != reflect.Invalid {
			return values[1].Interface().(error)
		}

		rspValue := values[0].Interface()
		if rpc.method == "Call" {
			rpcrsp := rspValue.(*kittyrpc.Response)
			if len(rpcrsp.Msg) > 0 {
				res := &kitty.CrudResult{}
				if err := json.Unmarshal([]byte(rpcrsp.Msg), res); err != nil {
					return err
				}
				if res.Code != 1 {
					return errors.New(res.Message)
				}
				obj, _ := json.Marshal(res.Data)
				rspValue = kitty.TypeKind(rpc.result).Create().Raw()
				if err := json.Unmarshal(obj, rspValue); err != nil {
					return fmt.Errorf("rpc call %s parse error %s", rpc.name, err.Error())
				}
			}
		}
		if err := rpc.result.Set(rspValue); err != nil {
			return err
		}
	}
	return nil
}
