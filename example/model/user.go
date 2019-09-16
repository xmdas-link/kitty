package model

import "time"

type User struct {
	ID         uint32 `gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string `gorm:"UNIQUE_INDEX"`
	Age        int
	Department string
}

