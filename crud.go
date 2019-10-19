package kitty

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Knetic/govaluate"
	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/fatih/structs"
	"github.com/jinzhu/gorm"
)

//CRUDInterface ...
type CRUDInterface interface {
	Do(*SearchCondition, string, Context) (interface{}, error)
}

// SuccessCallback 执行成功后，回调。返回error后，回滚事务
type SuccessCallback func(*Structs, *gorm.DB) error

// CRUD配置
type config struct {
	strs   *Structs         //模型结构
	search *SearchCondition //查询条件
	db     *gorm.DB         //db
	ctx    Context          //上下文
	callbk SuccessCallback  //成功回调
}

type crud struct {
	*config
}

func newcrud(conf *config) *crud {
	return &crud{conf}
}
func (crud *crud) queryExpr() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
	)

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}

	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

	kittys := &kittys{
		ctx:          c,
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(s); err != nil {
		return nil, err
	}

	var qry qry
	//	if len(kittys.kittys) > 1 {
	qry = evalJoin(s, kittys, search, db)
	//} else {
	//	qry = evalSimpleQry(s, kittys, search, db)
	//}
	return qry.prepare().QueryExpr(), nil
}

func (crud *crud) queryObj() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
		callbk = crud.callbk
	)

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}

	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

	Page := &Page{}
	if f, ok := s.FieldOk("Page"); ok {
		Page.Page = f.Value().(uint32)
	}
	if f, ok := s.FieldOk("Limit"); ok {
		Page.Limit = f.Value().(uint32)
	}
	if Page.Limit > 0 && Page.Page > 0 {
		search.Page = Page
	}

	kittys := &kittys{
		ctx:          c,
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(s); err != nil {
		return nil, err
	}

	var qry qry
	//if len(kittys.kittys) > 1 {
	qry = evalJoin(s, kittys, search, db)
	//	} else {
	//	qry = evalSimpleQry(s, kittys, search, db)
	//}

	var (
		res interface{}
		err error
	)
	res, err = execqry(qry, kittys.multiResult)
	if err != nil || res == nil {
		return nil, err
	}

	if len(kittys.resultField) > 0 {
		if err = s.Field(kittys.resultField).Set(res); err != nil {
			return nil, err
		}
	}

	params := make(map[string]interface{})
	params["ms"] = s
	params["kittys"] = kittys
	if err = setter(s, params, db, c); err != nil {
		return nil, err
	}

	if callbk != nil {
		if err = callbk(s, db); err != nil {
			return nil, err
		}
	}

	if f, ok := s.FieldOk("Pages"); ok && search.Page != nil {
		f.Set(search.Page)
	}

	return s.raw, nil
}

// CreateObj ...
func (crud *crud) createObj() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
		callbk = crud.callbk
	)

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}

	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

	kittys := &kittys{
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(s); err != nil {
		return nil, err
	}

	qry := &simpleQuery{
		db:           db,
		ModelStructs: s,
		search:       search,
		Result:       kittys.master().structs,
	}
	for _, v := range kittys.kittys {
		if !v.Master {
			qry.Next = append(qry.Next, &simpleQuery{
				db:           db,
				ModelStructs: s,
				search:       &SearchCondition{},
				Result:       v.structs,
			})
		}
	}
	res, err := qry.create()
	if err != nil {
		return nil, err
	}
	for _, v := range kittys.kittys {
		f := s.Field(v.FieldName)
		f.Set(v.structs.raw)
	}

	if len(kittys.resultField) > 0 {
		if err = s.Field(kittys.resultField).Set(res); err != nil {
			return nil, err
		}
	}

	params := make(map[string]interface{})
	params["ms"] = s
	params["kittys"] = kittys
	if err = setter(s, params, db, c); err != nil {
		return nil, err
	}
	if callbk != nil {
		if err = callbk(s, db); err != nil {
			return nil, err
		}
	}

	return s.raw, nil
}

func (crud *crud) updateObj() (interface{}, error) {
	var (
		s      = crud.strs
		search = crud.search
		db     = crud.db
		c      = crud.ctx
		callbk = crud.callbk
	)

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}

	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

	kittys := &kittys{
		ModelStructs: s,
		db:           db,
	}
	if err := kittys.parse(s); err != nil {
		return nil, err
	}

	qry := &simpleQuery{
		db:           db,
		ModelStructs: s,
		search:       search,
		Result:       kittys.master().structs,
	}
	for _, v := range kittys.kittys {
		if !v.Master {
			qry.Next = append(qry.Next, &simpleQuery{
				db:           db,
				ModelStructs: s,
				search:       &SearchCondition{},
				Result:       v.structs,
			})
		}
	}

	if err := qry.update(); err != nil {
		return nil, err
	}
	params := make(map[string]interface{})
	params["ms"] = s
	params["kittys"] = kittys
	if err := setter(s, params, db, c); err != nil {
		return nil, err
	}

	if callbk != nil {
		if err := callbk(s, db); err != nil {
			return nil, err
		}
	}

	return s.raw, nil
}

//
func queryObj(s *Structs, search *SearchCondition, db *gorm.DB, c Context) (interface{}, error) {
	crud := newcrud(&config{
		strs:   s,
		search: search,
		db:     db,
		ctx:    c,
	})
	return crud.queryObj()
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
func (crud *crud) execRPC() (interface{}, error) {
	var (
		s          = crud.strs
		db         = crud.db
		c          = crud.ctx
		ctx        = crud.ctx.GetCtx()
		callbk     = crud.callbk
		rpcClients = crud.search.Params
	)

	type rpc struct {
		name        string
		client      *Structs
		method      string
		methodField string
		param       *Structs
		result      *structs.Field
	}

	if err := vd.Validate(s.raw); err != nil {
		return nil, err
	}
	if err := getter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

	rpcs := make([]*rpc, 0)
	var getrpc = func(client string, method string) *rpc {
		for _, v := range rpcs {
			if v.name == client && v.method == method {
				return v
			}
		}
		return nil
	}

	for _, f := range s.Fields() {
		if k := f.Tag("kitty"); len(k) > 0 {
			tk := TypeKind(f)
			if call := GetSub(k, "call"); len(call) > 0 {
				// UsersRsp  *userProto.UsersResponse  `json:"-" kitty:"call:userClient.GetUsers"`
				v := strings.Split(call, ".")
				rpcs = append(rpcs, &rpc{
					name:   v[0],
					client: CreateModelStructs(rpcClients[v[0]]),
					method: v[1],
					result: f,
				})
			}
			if protocol := GetSub(k, "protocol"); len(protocol) > 0 {
				// GetRequest   *schoolProto.GetRequest  `json:"-" kitty:"protocol:schoolClient.GetSchool"`
				methodStrs := tk.create()
				if err := f.Set(methodStrs.raw); err != nil {
					return nil, err
				}
				v := strings.Split(protocol, ".")
				rpc := getrpc(v[0], v[1])
				rpc.methodField = f.Name()
				rpc.param = methodStrs
			}
		}
	}

	for _, rpc := range rpcs {
		//	SchoolId      *uint             `json:"-" kitty:"param:PageRequest.SchoolId;runtime:set(0)"`
		paramformat := fmt.Sprintf("%s.", rpc.methodField)
		for _, f := range s.Fields() {
			if k := f.Tag("kitty"); len(k) > 0 {
				if param := GetSub(k, "param"); len(param) > 0 && strings.Contains(param, paramformat) {
					if runtime := GetSub(k, "runtime"); len(runtime) > 0 {
						expr := &expr{
							s:         s,
							f:         f,
							functions: make(map[string]govaluate.ExpressionFunction),
							params:    make(map[string]interface{}),
						}
						expr.init()
						if err := expr.eval(runtime); err != nil {
							return nil, err
						}
					}
					v := strings.Split(param, ".")
					ff := rpc.param.Field(v[1])
					if err := rpc.param.SetFieldValue(ff, f.Value()); err != nil {
						return nil, err
					}
				}
			}
		}
		values := rpc.client.CallMethod(rpc.method, reflect.ValueOf(ctx), reflect.ValueOf(rpc.param.raw))

		if err := values[1].Interface().(error); err != nil {
			return nil, err
		}
		if err := rpc.result.Set(values[0].Interface()); err != nil {
			return nil, err
		}

	}
	if err := setter(s, make(map[string]interface{}), db, c); err != nil {
		return nil, err
	}

	if callbk != nil {
		if err := callbk(s, crud.db); err != nil {
			return nil, err
		}
	}
	return s.raw, nil
}
