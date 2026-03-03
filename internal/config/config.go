// Package config is responsible for loading environment variables from a .env file if it exists,
// or from the system environment variables if the .env file is not found.
// It uses the github.com/joho/godotenv package to handle this functionality.
package config

import (
	"log"

	"github.com/joho/godotenv"
)

// Init loads environment variables from a .env file if it exists, otherwise it relies on system environment variables.
func Init() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}
}
