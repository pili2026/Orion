package main

import (
	"log"
	"os"

	"github.com/hill/orion/internal/config"
	"github.com/hill/orion/internal/database"

	"github.com/hill/orion/internal/handler"
	"github.com/hill/orion/internal/service"
)

func main() {
	// 1. load configuration
	config.Init()

	// 2. Init database connection
	db := database.InitDB()

	// 3. Init MQTT client
	mqttClient := service.InitMQTT()

	// 4. Start HTTP server
	handler := handler.NewHandler(db, mqttClient)
	handler.SetupMQTTSubscribers()

	r := handler.SetupRouter()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
