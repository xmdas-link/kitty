package model

import (
	"github.com/jinzhu/gorm"
)

func AutoDB(db *gorm.DB) {
	db.AutoMigrate(&User{})
}
