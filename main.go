package main

import (
	"log"
	"codeiva/krono-api/app"
	"codeiva/krono-api/config"
)

func main() {
	config := config.GetConfig()

	a := &app.App{}
	a.Initialize(config)
	log.Printf("starting %s on %s", "krono-api", ":3000")
	a.Run(":3000")
}
