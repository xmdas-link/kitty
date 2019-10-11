package kitty_test

import (
	"testing"

	"github.com/Knetic/govaluate"
	"github.com/stretchr/testify/require"
	"github.com/xmdas-link/kitty"
)

func TestValidate(t *testing.T) {
	should := require.New(t)
	expression, err := govaluate.NewEvaluableExpression("foo >= 1")
	kitty.RegisterType(&User{})
	s := kitty.CreateModel("User")
	a := 1
	s.Field("Age").Set(a)

	parameters := make(map[string]interface{}, 8)
	parameters["foo"] = float64(10) //kitty.DereferenceValue(reflect.ValueOf(s.Field("Age").Value())).Interface()
	parameters["foo1"] = a
	parameters["foo2"] = &a

	result, err := expression.Evaluate(parameters)
	should.Nil(err)
	should.True(result.(bool))
}

func TestSetFieldValue(t *testing.T) {
	should := require.New(t)

	u := kitty.CreateModel("user")
	age := u.Field("Age")
	dep := u.Field("Department")
	tk := kitty.TypeKind(dep)
	should.Equal(dep.Kind(), tk.KindOfField)
	salary := u.Field("Salary")
	birthday := u.Field("Birthday")
	create := u.Field("CreatedAt")
	should.NoError(u.SetFieldValue(create, 1405544146))
	should.NoError(u.SetFieldValue(birthday, "2006-01-02 15:04:05"))
	should.NoError(u.SetFieldValue(salary, "11.11"))
	should.NoError(u.SetFieldValue(age, "11"))
	should.NoError(u.SetFieldValue(dep, 11))

}
