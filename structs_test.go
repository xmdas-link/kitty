package kitty_test

import (
	"reflect"
	"testing"

	"github.com/Knetic/govaluate"
	"github.com/stretchr/testify/require"
	"github.com/xmdas-link/kitty"
)

func TestValidate(t *testing.T) {
	should := require.New(t)
	expression, err := govaluate.NewEvaluableExpression("foo == 1")
	kitty.RegisterType(&User{})
	s := kitty.CreateModel("user")
	a := 1
	s.Field("Age").Set(&a)

	parameters := make(map[string]interface{}, 8)
	parameters["foo"] = kitty.DereferenceValue(reflect.ValueOf(s.Field("Age").Value())).Interface()
	parameters["foo1"] = a
	parameters["foo2"] = &a

	result, err := expression.Evaluate(parameters)
	should.Nil(err)
	should.False(result.(bool))
}

func TestSetFieldValue(t *testing.T) {
	should := require.New(t)

	u := kitty.CreateModel("user")
	age := u.Field("Age")
	dep := u.Field("Department")
	salary := u.Field("Salary")
	birthday := u.Field("Birthday")
	create := u.Field("CreatedAt")
	should.NoError(u.SetFieldValue(create, "1405544146"))
	should.NoError(u.SetFieldValue(birthday, "2006-01-02 15:04:05"))
	should.NoError(u.SetFieldValue(salary, "11.11"))
	should.NoError(u.SetFieldValue(age, "11"))
	should.NoError(u.SetFieldValue(dep, 11))

}

func TestQryFormat(t *testing.T) {
	should := require.New(t)

	type Test1 struct {
		Age        string `kitty:"param:user.age"`
		Age2       int    `kitty:"param:user.age"`
		Name       string `kitty:"param:user.name"`
		FindByName []string
	}

	kitty.RegisterType(&Test1{})
	s := kitty.CreateModel("Test1")
	f := s.Field("FindByName")
	should.Nil(s.SetFieldValue(f, []string{"1", "2"}))
	format := kitty.FormatQryField(f)
	should.Contains(format, "IN")

	f = s.Field("Name")
	should.Nil(s.SetFieldValue(f, "bill..ga,te,s"))
	format = kitty.FormatQryField(f)
	should.Contains(format, "=")

	should.Nil(s.SetFieldValue(f, "billgates%"))
	format = kitty.FormatQryField(f)
	should.Contains(format, "LIKE")

	f = s.Field("Age")
	should.Nil(s.SetFieldValue(f, "1,2,3"))
	format = kitty.FormatQryField(f)
	should.Contains(format, "IN")

	should.Nil(s.SetFieldValue(f, "1..3"))
	format = kitty.FormatQryField(f)
	should.Contains(format, "BETWEEN")

	f = s.Field("Age2")
	should.Nil(s.SetFieldValue(f, "99"))
	format = kitty.FormatQryField(f)
	should.Contains(format, "=")
}
