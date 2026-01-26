package main

import (
	"codeiva/krono-api/app"
	"codeiva/krono-api/config"
	"github.com/joho/godotenv"
	"log"
)

func main() {
	// Load .env file if exists
	if err := godotenv.Load(); err != nil {
		log.Printf(".env file not found, proceeding with environment variables")
	} else {
		log.Printf(".env file loaded successfully")
	}

	// Initialize configuration
	config := config.GetConfig()

	a := &app.App{}
	a.Initialize(config)
	log.Printf("starting %s on %s", "krono-api", ":3000")
	a.Run(":3000")
}
