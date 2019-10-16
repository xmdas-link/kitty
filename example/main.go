package main

import (
	"errors"

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
	web := web.NewLocalWeb(&web.Config{
		Model: &model.FormCreateUser{},
		DB:    DB,
		Ctx:   &currentCtx{},
	})
	r.POST("user/create", web.Create)
}

func FormUpdateUser(r *gin.RouterGroup) {
	web := web.NewLocalWeb(&web.Config{
		Model: &model.FormUpdateUser{},
		DB:    DB,
		Ctx:   &currentCtx{},
	})
	r.POST("user/update", web.Update)
}

func FormUser(r *gin.RouterGroup) {
	web := web.NewLocalWeb(&web.Config{
		Model: &model.FormUser{},
		DB:    DB,
		Ctx:   &currentCtx{},
	})
	r.GET("user/one", web.One)
}

func FormUserList(r *gin.RouterGroup) {
	web := web.NewLocalWeb(&web.Config{
		Model: &model.FormUserList{},
		DB:    DB,
		Ctx:   &currentCtx{},
	})
	r.GET("user/list", web.List)
}

type currentCtx struct {
}

func (*currentCtx) GetUID(ctx interface{}) (string, error) {
	//登录的信息存在gin的上下文。
	c := ctx.(*gin.Context)
	user := c.GetStringMapString("AuthUser")
	if uid, ok := user["id"]; ok {
		return uid, nil
	}
	return "", errors.New("nothing")
}

func (*currentCtx) GetCtxInfo(ctx interface{}, s string) (string, error) {
	return "", nil
}
