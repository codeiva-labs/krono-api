package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	config := config.GetConfig()

	app := &app.App{}
	app.Initialize(config)
	app.Run(":3000")
}
