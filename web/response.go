package web

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xmdas-link/filter"
)

type webResponse interface {
	success(interface{})
	fail(error)
}

type ginResponse struct {
	C *gin.Context
}

func (c *ginResponse) success(data interface{}) {
	c.C.JSON(http.StatusOK, filter.H{Ctx: c.C, Data: data})
}

func (c *ginResponse) fail(err error) {
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
func (c *nativeResponse) success(data interface{}) {
	c.write(data)
}

func (c *nativeResponse) fail(err error) {
	res := map[string]interface{}{
		"code": 0,
		"message":  err.Error(),
	}
	c.write(res)
}
