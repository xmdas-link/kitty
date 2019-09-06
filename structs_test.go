package kitty_test

import (
	"net/url"
	"testing"

	"dcx.com/1/model"
	"dcx.com/1/kitty"
	"github.com/stretchr/testify/require"
)

func TestParseFormValue(t *testing.T) {
	should := require.New(t)
	url := make(url.Values)
	url["product_name"] = []string{"go"}
	url["amount"] = []string{"2", "1"}
	url["buy_time"] = []string{"2019-10-09", "2019-11-29"}

	obj := &model.Orders{}
	ss := kitty.NewStr(obj)
	err := ss.ParseFormValues(url)
	should.Nil(err)


	
//	field := &utils.FormField{ss.Field("Amount")}
//	q := field.ToQuery()
//	should.Nil(err)

//ss.BuildFormQuery("order_items", "order_item")
}
