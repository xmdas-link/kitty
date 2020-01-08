package web

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/filter"
	"github.com/xmdas-link/kitty"
)

// WebResponse default json output
type WebResponse interface {
	Response(kitty.Context, interface{}, error)
}

type ginResponse struct {
	C *gin.Context
}

func (c *ginResponse) Response(e kitty.Context, data interface{}, err error) {
	if err != nil {
		c.Fail(err)
		return
	}
	c.C.JSON(http.StatusOK, filter.H{Ctx: c.C, Data: data})
}

func (c *ginResponse) Fail(err error) {
	c.C.JSON(http.StatusOK, gin.H{"code": 0, "message": err.Error()})
}

type nativeResponse struct {
	W http.ResponseWriter
}

func (c *nativeResponse) write(data interface{}) {
	c.W.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate, value")
	c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(c.W).Encode(data)
}
func (c *nativeResponse) Response(e kitty.Context, data interface{}, err error) {
	if err != nil {
		c.Fail(err)
		return
	}
	c.write(data)
}

func (c *nativeResponse) Fail(err error) {
	res := map[string]interface{}{
		"code":    0,
		"message": err.Error(),
	}
	c.write(res)
}
