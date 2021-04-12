package server

import (
	"github.com/inburst/prty/api/handlers"

	"github.com/gin-gonic/gin"
)

func Listen() *gin.Engine {
	r := gin.Default()
	registerHandlers(r)
	r.Run()
	return r
}

func registerHandlers(r *gin.Engine) {
	r.GET("/ping", handlers.HandlePing)
}
