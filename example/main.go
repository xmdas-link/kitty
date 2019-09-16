package main

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/xmdas-link/kitty"
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
	res := kitty.NewResource(&model.FormCreateUser{}, "user")
	web := &web.WebCrud{Resource: res, Crud: &kitty.LocalCrud{
		Model: res.ModelName,
		DB:    DB,
	}}
	r.POST(web.RoutePath()+"/create", web.Create)
}

func FormUpdateUser(r *gin.RouterGroup) {
	res := kitty.NewResource(&model.FormUpdateUser{}, "user")
	web := &web.WebCrud{Resource: res, Crud: &kitty.LocalCrud{
		Model: res.ModelName,
		DB:    DB,
	}}
	r.POST(web.RoutePath()+"/update", web.Update)
}

func FormUser(r *gin.RouterGroup) {
	res := kitty.NewResource(&model.FormUser{}, "user")
	web := &web.WebCrud{Resource: res, Crud: &kitty.LocalCrud{
		Model: res.ModelName,
		DB:    DB,
	}}
	r.GET(web.RoutePath()+"/one", web.One)
}

func FormUserList(r *gin.RouterGroup) {
	res := kitty.NewResource(&model.FormUserList{}, "user")
	web := &web.WebCrud{Resource: res, Crud: &kitty.LocalCrud{
		Model: res.ModelName,
		DB:    DB,
	}}
	r.GET(web.RoutePath()+"/list", web.List)
}
