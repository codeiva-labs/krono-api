package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"

	"codeiva/krono-api/app"
	"codeiva/krono-api/config"
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

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	a := &app.App{}
	a.Initialize(config)
	log.Printf("starting %s on %s", "krono-api", ":"+port)
	a.Run(":" + port)
}
