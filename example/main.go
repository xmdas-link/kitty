package main

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/xmdas-link/kitty/example/model"
	"github.com/xmdas-link/kitty/web"
)

var (
	DB *gorm.DB
)

func main() {
	db, err := gorm.Open("sqlite3", "test.db")
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()
	db.LogMode(true)

	model.AutoDB(db)

	DB = db

	c := gin.Default()
	r := c.Group("test")

	FormCreateUser(r)
	FormUpdateUser(r)
	FormUser(r)
	FormUserList(r)

	c.Run()

}

func FormCreateUser(r *gin.RouterGroup) {
	web := web.NewWeb(&model.FormCreateUser{}, "user", DB, &currentCtx{})
	r.POST(web.RoutePath()+"/create", web.Create)
}

func FormUpdateUser(r *gin.RouterGroup) {
	web := web.NewWeb(&model.FormUpdateUser{}, "user", DB, &currentCtx{})
	r.POST(web.RoutePath()+"/update", web.Update)
}

func FormUser(r *gin.RouterGroup) {
	web := web.NewWeb(&model.FormUser{}, "user", DB, &currentCtx{})
	r.GET(web.RoutePath()+"/one", web.One)
}

func FormUserList(r *gin.RouterGroup) {
	web := web.NewWeb(&model.FormUserList{}, "user", DB, &currentCtx{})
	r.GET(web.RoutePath()+"/list", web.List)
}

type currentCtx struct {
}

func (*currentCtx) GetUID(ctx interface{}) string {
	//登录的信息存在gin的上下文。
	c := ctx.(*gin.Context)
	user := c.GetStringMapString("AuthUser")
	if uid, ok := user["id"]; ok {
		return uid
	}
	return ""
}
