package kitty

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
)

// kittys ...
type kittys struct {
	ModelStructs *Structs
	db           *gorm.DB
	kittys       []*kitty
	binds        []*fieldBinding
	result       *Structs
	multiResult  bool
}

// Parse ...
func (ks *kittys) parse() error {
	for _, f := range ks.ModelStructs.Fields() {
		fmt.Printf("field name: %+v\n", f.Name())
		if k := f.Tag("kitty"); len(k) > 0 && !strings.Contains(k, "bind") {
			if f.Kind() == reflect.Struct {
				modelName := ToCamel(reflect.TypeOf(f.Value()).Name())
				kitty := &kitty{ModelStructs: ks.ModelStructs}
				kitty.parse(k, modelName, f.Name(), ks.db)
				ks.kittys = append(ks.kittys, kitty)
				//if !ks.master().Master {
				//	return fmt.Errorf("第一个结构体必须是标识master标签")
				//}
			}
		}
	}
	for _, f := range ks.ModelStructs.Fields() {
		k := f.Tag("kitty")
		if strings.Contains(k, "bindresult") {
			tk := (&FormField{f}).TypeAndKind()
			ks.result = ks.ModelStructs.createModelStructs(tk.ModelName) //NewModelStruct(tk.ModelName)
			for k, v := range ks.ModelStructs.strTypes {
				ks.result.strTypes[k] = v
			}
			ks.multiResult = tk.TypeOfField.Kind() == reflect.Slice

			if kkkk := ks.get(tk.ModelName); kkkk != nil {
				binding := kkkk.parse(k, tk.ModelName, f.Name(), ks.db)
				ks.binds = append(ks.binds, binding)
			} else {
				kbind := &kittys{
					db:           ks.db,
					ModelStructs: ks.result,
				}
				if err := kbind.parse(); err != nil {
					return err
				}
				ks.binds = append(ks.binds, kbind.binds...)
			}

		} else if strings.Contains(k, "bind") {
			modelField := GetSub(k, "bind")
			modelName := ToCamel(strings.Split(modelField, ".")[0])
			kitty := &kitty{ModelStructs: ks.ModelStructs}
			binding := kitty.parse(k, modelName, f.Name(), ks.db)
			ks.binds = append(ks.binds, binding)
		}
	}
	return nil
}
func (ks *kittys) check() error {
	for _, bind := range ks.binds {
		if ks.get(bind.ModelName) == nil {
			return fmt.Errorf("model %s not declare", bind.ModelName)
		}
	}
	return nil
}

// master 主表 第一个Struct肯定标识为master
func (ks *kittys) master() *kitty {
	if len(ks.kittys) > 0 {
		return ks.kittys[0]
	}
	return nil
}

func (ks *kittys) get(modelname string) *kitty {
	for _, v := range ks.kittys {
		if v.ModelName == modelname {
			return v
		}
	}
	return nil
}
func (ks *kittys) selects() []string {
	s := []string{}
	for _, v := range ks.binds {
		s = append(s, v.selectAs())
	}
	return s
}
func (ks *kittys) joins() []*fieldQryFormat {
	s := []*fieldQryFormat{}
	for _, v := range ks.kittys {
		if !v.Master {
			s = append(s, v.joins(ks.ModelStructs, ks.get(v.JoinTo)))
		}
	}
	return s
}

func (ks *kittys) where() []*fieldQryFormat {
	s := []*fieldQryFormat{}
	if query := ks.ModelStructs.buildFormQuery(ks.master().TableName, ks.master().ModelName); len(query) > 0 {
		s = append(s, query...)
	}
	for _, bind := range ks.binds {
		if bind.Having { //过滤having字段
			continue
		}
		if q := ks.ModelStructs.buildFormFieldQuery(bind.FieldName); q != nil {
			q.field = bind.funcName() + " " + q.field
			s = append(s, q)
		}
	}
	return s
}

func (ks *kittys) groupby() []string {
	s := []string{}
	for _, v := range ks.kittys {
		s = append(s, v.groupBy()...)
	}
	return s
}
func (ks *kittys) having() *fieldQryFormat {
	for _, bind := range ks.binds {
		if bind.Having {
			if q := ks.ModelStructs.buildFormParamQuery(ks.result.Name(), bind.FieldName); q != nil {
				q.field = bind.funcName() + " " + q.field
				// having 的统计是不是都应该是整型值？0917
				for i, v := range q.v {
					switch v.(type) {
					case string:
						x, _ := strconv.ParseInt(v.(string), 10, 64)
						q.v[i] = reflect.ValueOf(x).Interface()
					}
				}
				return q //sum(xxx) > 50
			}
		}
	}
	return nil
}

func (ks *kittys) subWhere(model string) []*fieldQryFormat {
	for _, bind := range ks.binds {
		if bind.ModelName == ToCamel(model) {
			if q := ks.ModelStructs.buildFormQuery(bind.TableName, bind.ModelName); q != nil {
				return q
			}
		}
	}
	return nil
}
