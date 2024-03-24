package main

import (
	"log"
	"net/http"

	limits "github.com/gin-contrib/size"
	"github.com/gin-gonic/gin"
)

func handler(ctx *gin.Context) {
	val := ctx.PostForm("b")
	if len(ctx.Errors) > 0 {
		return
	}
	ctx.String(http.StatusOK, "got %s\n", val)
}

func main() {
	r := gin.Default()
	r.Use(limits.RequestSizeLimiter(10))
	r.POST("/", handler)
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
