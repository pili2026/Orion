// Package handler sets up the HTTP router for the application using the Gin framework.
package handler

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler struct holds the database connection and MQTT client for use in route handlers.
type Handler struct {
	DB   *gorm.DB
	MQTT mqtt.Client
}

// NewHandler creates a new Handler instance with the provided database connection and MQTT client.
func NewHandler(db *gorm.DB, mqttClient mqtt.Client) *Handler {
	return &Handler{
		DB:   db,
		MQTT: mqttClient,
	}
}

func (h *Handler) SetupRouter() *gin.Engine {
	r := gin.Default()

	// Replace with actual trusted proxies if needed, or set to nil to trust all (not recommended for production)
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("Failed to set trusted proxies: %v", err)
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 測試：透過 API 發送控制指令給 Edge 設備
	r.POST("/publish", func(c *gin.Context) {
		// 指定發送到 Talos 的指令頻道
		topic := "talos/command/edge_device_002"

		// 模擬一個要叫設備重啟或開關的 JSON 指令
		payload := `{"action": "turn_on_fan", "speed": 100}`

		// 使用 DI 注入的 MQTT Client 發送訊息 (QoS 1)
		token := h.MQTT.Publish(topic, 1, false, payload)
		token.Wait()

		if token.Error() != nil {
			c.JSON(500, gin.H{"error": "指令發送失敗: " + token.Error().Error()})
			return
		}

		c.JSON(200, gin.H{
			"message": "指令已成功發送給 Edge 設備！",
			"topic":   topic,
			"payload": payload,
		})
	})

	return r
}
