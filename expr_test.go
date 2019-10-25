package kitty_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/xmdas-link/kitty"
)

type User struct {
	ID         uint32    `gorm:"primary_key;AUTO_INCREMENT" json:"id,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	Name       string    `gorm:"UNIQUE_INDEX" form:"type:text;" permission:"rw:admin; r:api" json:"name,omitempty"`
	Age        uint      `json:"age,omitempty"`
	Department string
	Manager    int
	Birthday   time.Time
	Salary     float64
	Online     bool
}

type Company struct {
	ID        uint32 `gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Industry  int    `gorm:"DEFAULT:0"`
	Name      string `permission:"rw:admin; r:users,api"`
	Job       string
	UserID    uint32
}

type Department struct {
	ID        uint32 `gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	CompanyID uint32
}

type CreditCard struct {
	ID        uint32 `gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Number    string
	UserID    uint32
}

type Language struct {
	Name   string `gorm:"primary_key:true;UNIQUE;INDEX"`
	UserID uint32
}

type UserResult struct {
	Name       string
	Department string
	MyAge      float64 `alias:"age"`
}

type Test struct {
	UserSlice  []*User
	UserSlice2  []*User
	UserResult []*UserResult
	User1      *User
	User       *User
	Name       string
	Age        int
	Active     float64
	UserName   string
	Company    *Company
	FindByName []string
	Names      *interface{}
	Ages       *interface{}
}

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open("sqlite3", "test.db")
	if err != nil {
		panic("failed to connect database")
	}

	db.LogMode(true)
	db.AutoMigrate(&User{}, &Company{}, &CreditCard{}, &Language{})

	db.Delete(&User{})
	db.Delete(&Company{})
	kitty.RegisterType(&Test{})
	kitty.RegisterType(&User{})
	kitty.RegisterType(&Company{})
	kitty.RegisterType(&CreditCard{})
	kitty.RegisterType(&Language{})
}
func TestExpr(t *testing.T) {
	defer db.Close()
	should := require.New(t)

	v := &Test{}
	s := kitty.CreateModelStructs(v)

	s.Field("User1").Set(&User{})
	s.Field("User1").Field("Name").Set("huang")
	s.Field("User1").Field("Age").Set(uint(10))

	kitty.RegisterFunc("split", func(args ...interface{}) (interface{}, error) {
		if args[0] == nil {
			return nil, nil
		}
		return strings.Split(args[0].(string), args[1].(string)), nil
	})
	kitty.RegisterFunc("test", func(args ...interface{}) (interface{}, error) {
		return 99, nil
	})

	should.Error(kitty.Eval(s, db, s.Field("User1"), "vf(this.name== 'bill'?'name should huang')"))
	//	should.Nil(kitty.Eval(s, db, s.Field("User1"), "vf(this.age==10?'error iii')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "vf(this==nil?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=user1.name,age=user1.age,department=dev')|vf(this.name=='huang'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create_if(this.name=='huang'?'name=[bill],age=[20],department=[sales]')|vf(this.name=='bill'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_update('name=billgates,age=0','name=bill')"))
	should.Nil(kitty.Eval(s, db, s.Field("Company"), "vf(company==nil?'error')|rd_create('name=oracle,job=hr,user_id=user.id')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "vf(company!=nil?'error')|rd_update_if(company!=nil?'department=company.name','name=billgates')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice"), "rds()|vf(len(this)==2?'xxx')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice2"), "f('user_slice')|vf(len(this)==2?'xxx')"))
	should.Nil(kitty.Eval(s, db, s.Field("FindByName"), "f('user_slice[*].name')|vf(len(this)==2?'xxx')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserResult"), "f('user_slice')|vf(len(this)==2&&user_result[0].name=='huang'?'error1')"))
	should.Nil(kitty.Eval(s, db, s.Field("User1"), "f('user_slice[0]')|vf(this!=nil&&this.name=='huang'?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rds('name=huang')|vf(len(split(this.name,','))==1?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "f('user.name')|vf(len(this)>0?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "set('hello,world')|vf(len(split(this,','))==2?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "rds('id=user.id','user.department')|vf(this=='dev'?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Age"), "set_if(user_slice[0].name=='huang'?test())|vf(this==99?sprintf('err:%d',f('age')))"))

	should.Nil(kitty.Eval(s, db, s.Field("Ages"), "rds('','user.age')|vf(this!=nil?'err')"))
	should.Nil(kitty.Eval(s, db, s.Field("Age"), "count(f('ages'))|vf(this==2?'sss')"))

}
func TestVf(t *testing.T) {
	defer db.Close()
	should := require.New(t)
	s := kitty.CreateModel("Test")
	s.Field("User1").Set(&User{})
	s.Field("User1").Field("Name").Set("huang")
	s.Field("User1").Field("Age").Set(10)
	s.SetFieldValue(s.Field("User1").Field("Age"), 10)
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=huang,age=10,department=dev')|vf(this.name=='huang'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=bill,age=20,department=sales')|vf(this.name=='bill'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "vf(this.age==20?'error iii')"))
	should.Error(kitty.Eval(s, db, s.Field("User1"), "vf(this.name== 'bill'?'name should huang')"))
	should.Nil(kitty.Eval(s, db, s.Field("User1"), "vf(this.age<user.age?'error iii')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice"), "rds()|vf(len(this)==2?'error1')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice"), "vf(user_slice[1].name=='bill'?'error1')"))
}

func TestF(t *testing.T) {
	defer db.Close()
	should := require.New(t)
	s := kitty.CreateModel("Test")
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=huang,age=10,department=dev')|vf(this.name=='huang'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=bill,age=20,department=sales')|vf(this.name=='bill'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "f('user.name')|vf(len(this)>0?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice"), "rds()|vf(len(this)==2?'error1')"))
	should.Nil(kitty.Eval(s, db, s.Field("User1"), "f('user_slice[0]')|vf(this!=nil&&this.name=='huang'?'error')"))
}

func TestCreate(t *testing.T) {
	defer db.Close()
	should := require.New(t)
	s := kitty.CreateModel("Test")
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=huang,age=10,department=dev')|vf(this.name=='huang'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=bill,age=20,department=sales')|vf(this.name=='bill'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create_if(this.name=='bill'?'name=billgates,age=30,department=sales')|vf(this.name=='billgates'?'errorr')"))
}

func TestRds(t *testing.T) {
	defer db.Close()
	should := require.New(t)
	s := kitty.CreateModel("Test")
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=huang,age=10,department=dev')|vf(this.name=='huang'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=bill,age=20,department=sales')|vf(this.name=='bill'?'errorr')"))
	// 多条记录
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice"), "rds()|vf(len(this)==2?'error1')"))
	// 单条记录
	should.Nil(kitty.Eval(s, db, s.Field("User1"), "rds('name=bill')|vf(this.age>=10?'error2')"))
	// scan到另外一个model
	should.Nil(kitty.Eval(s, db, s.Field("UserResult"), "rds('','user.*')|vf(len(this)==2&&user_result[0].name=='huang'?'error1')"))
	// 获取单列
	should.Nil(kitty.Eval(s, db, s.Field("FindByName"), "rds('','user.name')|vf(len(this)==2?'error1')"))
}
func TestSet(t *testing.T) {
	kitty.RegisterFunc("split", func(args ...interface{}) (interface{}, error) {
		if args[0] == nil {
			return nil, nil
		}
		return strings.Split(args[0].(string), args[1].(string)), nil
	})
	should := require.New(t)
	s := kitty.CreateModel("Test")
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "set('hello,world')|vf(len(split(this,','))==2?'error')"))
}

func TestQryExpr(t *testing.T) {
	defer db.Close()
	should := require.New(t)
	s := kitty.CreateModel("Test")

	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=huang,age=10,department=dev')|vf(this.name=='huang'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=bill,age=20,department=sales')|vf(this.name=='bill'?'errorr')"))
	s.Field("Name").Set("bill%")
	should.Nil(kitty.Eval(s, db, s.Field("Names"), "rds('name LIKE name','user.name')|vf(this!=nil?'err')"))
	v := s.Field("Names").Value()
	v = reflect.ValueOf(v).Elem().Interface()
	var users []User
	db.Select("*").Where("name IN (?)", v).Find(&users)
	//	db.Select("*").Where("name IN (?)", db.
	//		Select("name").Table("users").QueryExpr()).Find(&users)
	should.Len(users, 1)

	should.Nil(kitty.Eval(s, db, s.Field("Ages"), "rds('age>11,age<23','user.age')|vf(this!=nil?'err')"))

	v = s.Field("Ages").Value()
	v = reflect.ValueOf(v).Elem().Interface()
	var users2 []User
	db.Select("*").Where("age IN (?)", v).Find(&users2)
	//	db.Select("*").Where("name IN (?)", db.
	//		Select("name").Table("users").QueryExpr()).Find(&users)
	should.Len(users2, 1)

}

func TestBatchCreate(t *testing.T) {

}
