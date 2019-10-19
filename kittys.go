package kitty

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"

	"github.com/jinzhu/gorm"
)

// kittys ...
type kittys struct {
	ctx          Context //上下文
	ModelStructs *Structs
	db           *gorm.DB
	kittys       []*kitty
	binds        []*fieldBinding
	result       *Structs
	resultField  string
	multiResult  bool
	qryFormats   []*fieldQryFormat
}

func isKitty(ms *Structs) bool {
	for _, f := range ms.Fields() {
		if k := f.Tag("kitty"); len(k) > 0 {
			return true
		}
	}
	return false
}

// Parse ...
func (ks *kittys) parse(ms *Structs) error {
	for _, f := range ks.ModelStructs.Fields() {
		fmt.Printf("field name: %+v\n", f.Name())
		if k := f.Tag("kitty"); len(k) > 0 && (strings.Contains(k, "master") || strings.Contains(k, "join")) {
			if f.Kind() == reflect.Struct {
				tk := TypeKind(f)
				strs := tk.create()
				modelName := tk.ModelName
				kitty := &kitty{
					ModelStructs: ks.ModelStructs,
					ModelName:    modelName,
					FieldName:    f.Name(),
					structs:      strs,
					//	TableName:    ks.db.NewScope(strs.raw).TableName(),
				}
				if !isKitty(strs) {
					kitty.TableName = ks.db.NewScope(strs.raw).TableName()
				}
				kitty.parse(k, modelName, f.Name())
				ks.kittys = append(ks.kittys, kitty)
			}
		}
	}
	for _, f := range ks.ModelStructs.Fields() {
		k := f.Tag("kitty")
		tk := TypeKind(f)
		if strings.Contains(k, "bindresult") {
			ks.resultField = f.Name()
			ks.result = tk.create()
			ks.multiResult = tk.KindOfField == reflect.Slice
			if kkkk := ks.get(tk.ModelName); kkkk != nil {
				binding := kkkk.binding(k, tk.ModelName, f.Name())
				ks.binds = append(ks.binds, binding)
			} else {
				kbind := &kittys{
					db:           ks.db,
					ModelStructs: ks.result,
				}
				if err := kbind.parse(ms); err != nil {
					return err
				}
				for _, v := range kbind.binds {
					v.strs = ks.result
				}
				ks.binds = append(ks.binds, kbind.binds...)
			}

		} else if strings.Contains(k, "bind") {
			modelField := GetSub(k, "bind")
			kitty := &kitty{
				ModelStructs: ks.ModelStructs,
				FieldName:    f.Name(),
			}
			if strings.Contains(modelField, "(") && strings.Contains(modelField, ")") {
				kitty.ModelName = tk.ModelName
			} else {
				modelName := ToCamel(strings.Split(modelField, ".")[0])
				var strs *Structs
				if modelName == tk.ModelName {
					strs = tk.create()
				} else {
					strs = ms.createModel(modelName)
				}
				kitty.ModelName = modeName
				if !isKitty(strs) {
					kitty.TableName = ks.db.NewScope(strs.raw).TableName()
				}
			}
			binding := kitty.binding(k, kitty.ModelName, f.Name())
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

func (ks *kittys) prepare() {
	ks.qryFormats = ks.ModelStructs.buildAllParamQuery()

	if ks.result != nil {
		for _, v := range ks.qryFormats {
			if f, ok := ks.result.FieldOk(ToCamel(v.bindfield)); ok {
				if k := f.Tag("kitty"); strings.Contains(k, "format") {
					v.format = GetSub(k, "format")
				}
			}
		}
	}
}

func (ks *kittys) selects() *fieldQryFormat {
	s := []string{}
	value := []interface{}{}
	for _, v := range ks.binds {
		format := v.selects(ks.ModelStructs, ks.db)
		s = append(s, format.bindfield)
		value = append(value, format.value...)
	}
	return &fieldQryFormat{
		bindfield: strings.Join(s, ", "),
		value:     value,
	}
}

func (ks *kittys) joins() []*fieldQryFormat {
	s := []*fieldQryFormat{}
	for _, v := range ks.kittys {
		if !v.Master {
			//			if isKitty(v.structs) {
			//				s = append(s, v.joinKitty(ks.ModelStructs, ks.get(v.JoinTo), ks.db, ks.ctx))
			//			} else {
			s = append(s, v.joins(ks.ModelStructs, ks.get(v.JoinTo)))
			//			}
		}
	}
	return s
}

func (ks *kittys) where() []*fieldQryFormat {
	s := []*fieldQryFormat{}

	masterModel := strcase.ToSnake(ks.master().ModelName)
	tblname := strcase.ToSnake(ks.master().TableName)
	for _, v := range ks.qryFormats {
		if masterModel == v.model && len(v.format) == 0 && !v.order { // 带有format 约束的，放入having
			s = append(s, &fieldQryFormat{
				operator: fmt.Sprintf("%s.%s %s", tblname, v.bindfield, v.operator),
				value:    v.value,
			})
		}
	}

	for _, v := range ks.qryFormats {
		if masterModel != v.model && len(v.format) == 0 && !v.order {
			for _, bind := range ks.binds {
				fname := strcase.ToSnake(bind.FieldName)
				if fname == v.fname {
					s = append(s, &fieldQryFormat{
						operator: fmt.Sprintf("%s %s", bind.funcName(), v.operator),
						value:    v.value,
					})
				}
			}
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

func (ks *kittys) having() []*fieldQryFormat {
	s := []*fieldQryFormat{}
	for _, v := range ks.qryFormats {
		if len(v.format) > 0 && !v.order { // 带有format 约束的，放入having
			s = append(s, v)
		}
	}
	return s
}

func (ks *kittys) order() []*fieldQryFormat {
	s := []*fieldQryFormat{}
	for _, v := range ks.qryFormats {
		if v.order {
			if kit := ks.get(ToCamel(v.model)); kit != nil {
				v.bindfield = fmt.Sprintf("%s.%s", kit.TableName, v.bindfield)
			}
			//like users.id desc, count_issues asc
			s = append(s, v)
		}
	}
	return s
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
