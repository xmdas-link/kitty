package kitty

import (
	"testing"

	"dcx.com/1/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/stretchr/testify/require"
)

func TestKittys(t *testing.T) {
	should := require.New(t)
	db, err := gorm.Open("sqlite3", "test.db")
	should.Nil(err)

	kittys := &kittys{
		ModelStructs: NewStr(&model.FormProduct{}),
		db:           db,
	}
	err = kittys.parse()
	should.Nil(err)

	should.Nil(kittys.check())
}
