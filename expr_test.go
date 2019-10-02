package kitty_test

import (
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
	Age        int       `json:"age,omitempty"`
	Department string
	Manager    int
	Birthday   time.Time
	Salary     float64
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

type Test struct {
	UserSlice  []*User
	User1      *User
	User       *User
	Name       string
	Age        int
	Active     float64
	UserName   string
	Company    *Company
	FindByName []string
}

func init() {
	kitty.RegisterType(&Test{})
	kitty.RegisterType(&User{})
	kitty.RegisterType(&Company{})
	kitty.RegisterType(&CreditCard{})
	kitty.RegisterType(&Language{})
}
func TestExpr(t *testing.T) {
	should := require.New(t)

	db, err := gorm.Open("sqlite3", "test.db")
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()
	db.LogMode(true)
	db.AutoMigrate(&User{}, &Company{}, &CreditCard{}, &Language{})

	db.Delete(&User{})
	db.Delete(&Company{})

	s := kitty.CreateModel("Test")

	s.Field("User1").Set(&User{})
	s.Field("User1").Field("Name").Set("huang")
	s.Field("User1").Field("Age").Set(10)

	kitty.RegisterFunc("split", func(args ...interface{}) (interface{}, error) {
		if args[0] == nil {
			return nil, nil
		}
		return strings.Split(args[0].(string), args[1].(string)), nil
	})

	should.Error(kitty.Eval(s, db, s.Field("User1"), "vf(this.name== 'bill'?'name should huang')"))
	should.Nil(kitty.Eval(s, db, s.Field("User1"), "vf(this.age==10?'error iii')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "vf(this==nil?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create('name=user1.name,age=user1.age,department=dev')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_create_if(this.name=='huang'?'name=`bill`,age=`20`,department=`sales`')|vf(this.name=='bill'?'errorr')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd_update('name=`billgates`','name=bill')"))
	should.Nil(kitty.Eval(s, db, s.Field("Company"), "vf(company==nil?'error')|rd_create('name=oracle,job=hr,user_id=user.id')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "vf(company!=nil?'error')|rd_update_if(company!=nil?'department=company.name','name=billgates')"))
	should.Nil(kitty.Eval(s, db, s.Field("UserSlice"), "rds()"))
	should.Nil(kitty.Eval(s, db, s.Field("User1"), "f('user_slice[0]')|vf(this!=nil&&this.name=='huang'?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("User"), "rd('name=huang')|vf(len(split(this.name,','))==1?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "f('user.name')|vf(len(this)>0?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "set('hello,world')|vf(len(split(this,','))==2?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Name"), "db('user.department.id=user.id')|vf(this=='dev'?'error')"))
	should.Nil(kitty.Eval(s, db, s.Field("Age"), "set_if(user_slice[0].name=='huang'?'99')|vf(this==99?'error')"))
}
