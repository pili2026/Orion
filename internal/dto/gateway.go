package dto

// CreateGatewayRequest is the payload for POST /api/v1/gateways.
type CreateGatewayRequest struct {
	SiteID      string `json:"site_id"      binding:"required,uuid"`
	SerialNo    string `json:"serial_no"    binding:"required"`
	Mac         string `json:"mac"          binding:"required"`
	Model       string `json:"model"        binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	SSHPort     int    `json:"ssh_port"`
}

// UpdateGatewayRequest is the payload for PATCH /api/v1/gateways/:id.
// All fields are optional — only non-zero values are applied.
type UpdateGatewayRequest struct {
	DisplayName   *string `json:"display_name"`
	Model         *string `json:"model"`
	Status        *string `json:"status"`
	NetworkStatus *string `json:"network_status"`
	SSHPort       *int    `json:"ssh_port"`
}

// GatewayResponse is the standard gateway representation returned by the API.
type GatewayResponse struct {
	ID            string  `json:"id"`
	SiteID        string  `json:"site_id"`
	SerialNo      string  `json:"serial_no"`
	Mac           string  `json:"mac"`
	Model         string  `json:"model"`
	DisplayName   string  `json:"display_name"`
	Status        string  `json:"status"`
	NetworkStatus string  `json:"network_status"`
	SSHPort       int     `json:"ssh_port"`
	MQTTUsername  string  `json:"mqtt_username"`
	LastSeenAt    *string `json:"last_seen_at"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// RegisterGatewayResponse is returned only on POST /gateways.
// It includes the one-time MQTT password — it is never stored in the DB
// and cannot be retrieved again after this response.
type RegisterGatewayResponse struct {
	Gateway      GatewayResponse `json:"gateway"`
	MQTTPassword string          `json:"mqtt_password"`
	MQTTBroker   string          `json:"mqtt_broker"`
	MQTTPort     string          `json:"mqtt_port"`
}
