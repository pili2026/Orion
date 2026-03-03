package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
)

// 1. Create a mock Token to satisfy the mqtt.Token interface without doing any real work
type mockToken struct {
	mqtt.Token // Tips: We can embed the original interface to avoid implementing all its methods, since we only care about Wait() and Error() in this test
}

func (m *mockToken) Wait() bool   { return true }
func (m *mockToken) Error() error { return nil }

// 2. Create a mock MQTT client that records the topic and payload it was asked to publish, instead of sending real network requests
type MockMQTTClient struct {
	mqtt.Client // Same as above, we embed the original interface to avoid implementing all methods, since we only care about Publish() in this test

	// These fields will store the topic and payload that our test will check later
	PublishedTopic   string
	PublishedPayload interface{}
}

// Override the Publish method to capture the topic and payload instead of sending them to a real MQTT broker
func (m *MockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	m.PublishedTopic = topic
	m.PublishedPayload = payload
	return &mockToken{}
}

// 3. Now we can write our test function to verify that when we hit the /publish endpoint, it calls Publish() with the correct topic and payload
func TestPublishRoute(t *testing.T) {
	// Set Gin to Test Mode to avoid unnecessary output during testing
	gin.SetMode(gin.TestMode)

	// A. Create an instance of our Handler with the MockMQTTClient, and set up the router
	mockMQTT := &MockMQTTClient{}
	// Note: We can pass nil for the DB since our /publish route doesn't interact with the database in this test
	h := NewHandler(nil, mockMQTT)
	r := h.SetupRouter()

	// B. Prepare a test HTTP request to the /publish endpoint, and a ResponseRecorder to capture the response
	req, _ := http.NewRequest(http.MethodPost, "/publish", nil)
	w := httptest.NewRecorder() // This will capture the HTTP response for us to inspect later

	// C. Execute the request against our router
	r.ServeHTTP(w, req)

	// D. Assert the results:
	// Assertion 1: Check that we got a 200 OK response from the server
	if w.Code != http.StatusOK {
		t.Errorf("❌ Expected status code 200, got %d", w.Code)
	}

	// Assertion 2: Check that our MockMQTTClient's Publish method was called with the expected topic and payload
	if mockMQTT.PublishedTopic != "test/topic" {
		t.Errorf("❌ Expected published topic 'test/topic', got '%s'", mockMQTT.PublishedTopic)
	}

	expectedPayload := "Hello via DI!"
	if mockMQTT.PublishedPayload != expectedPayload {
		t.Errorf("❌ Expected published payload '%s', got '%v'", expectedPayload, mockMQTT.PublishedPayload)
	}
}
