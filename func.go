package kitty

import (
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/Knetic/govaluate"
	vd "github.com/bytedance/go-tagexpr/validator"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"
)

// ToCamel 对id的特别处理。 PayID UserID 。
func ToCamel(s string) string {
	r := strcase.ToCamel(s)
	if strings.ToLower(r) == "id" {
		r = "ID"
	} else if len(r) > 2 {
		sub := r[len(r)-2:]
		if strings.ToLower(sub) == "id" {
			return r[:len(r)-2] + "ID"
		}
	}
	return r
}

func makeSlice(elemType reflect.Type, len int) reflect.Value {
	if elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}
	sliceType := reflect.SliceOf(elemType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, len, len))
	return slice
}

// ptr wraps the given value with pointer: V => *V, *V => **V, etc.
func ptr(v reflect.Value) reflect.Value {
	pt := reflect.PtrTo(v.Type()) // create a *T type.
	pv := reflect.New(pt.Elem())  // create a reflect.Value of type *T.
	pv.Elem().Set(v)              // sets pv to point to underlying value of v.
	return pv
}

// DereferenceType dereference, get the underlying non-pointer type.
func DereferenceType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// DereferenceValue dereference and unpack interface,
// get the underlying non-pointer and non-interface value.
func DereferenceValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	return v
}

// GetSub 获得tag标签段
func GetSub(s string, sub string) string {
	if strings.Contains(s, sub) {
		v := strings.Split(s, ";")
		for _, v1 := range v {
			v2 := strings.Split(v1, ":") //bind:user.name
			if v2[0] == sub {
				return v2[1]
			}
		}
	}
	return ""
}

func pages(db *gorm.DB, search *SearchCondition, result interface{}, scan bool) (interface{}, error) {

	tx := db
	if search.Page != nil {
		// 参数检查
		if search.Page.Limit == 0 {
			return nil, errors.New("需指定每页返回个数")
		}

		total := 0
		if scan {
			rows, _ := tx.Rows()
			for rows.Next() {
				total++
			}
		} else {
			tx = tx.Count(&total)
		}

		// 页数
		pageInfo := MakePage(search.Page.Page, search.Page.Limit, uint32(total))
		search.Page.Limit = pageInfo.Limit
		search.Page.Page = pageInfo.Page
		search.Page.Total = uint32(total)
		search.Page.PageMax = pageInfo.PageMax

		tx = tx.Offset(pageInfo.GetOffset()).Limit(pageInfo.Limit)
	}

	if scan {
		tx = tx.Scan(result)
	} else {
		tx = tx.Find(result)
	}

	if tx.Error != nil {
		return nil, tx.Error
	}
	if tx.RecordNotFound() {
		return nil, nil
	}

	search.ReturnCount = int(tx.RowsAffected)

	return result, nil
}

var (
	exprFuncs map[string]govaluate.ExpressionFunction
)

func init() {
	exprFuncs = make(map[string]govaluate.ExpressionFunction)

	vd.RegFunc("time", func(args ...interface{}) bool {
		if len(args) != 1 {
			return false
		}
		s, ok := args[0].(string)
		if !ok {
			return false
		}
		_, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
		if err != nil {
			return false
		}
		return true
	}, true)

}

// RegisterFunc ...
func RegisterFunc(name string, func1 govaluate.ExpressionFunction) {
	exprFuncs[name] = func1
}

func trimSpace(s string) string {
	s = strings.TrimPrefix(s, " ")
	s = strings.TrimSuffix(s, " ")
	return s
}
