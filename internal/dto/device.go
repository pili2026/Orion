package dto

// UpdateDeviceRequest is the payload for PATCH /api/v1/devices/:id.
// All fields are optional — only explicitly provided fields are applied.
type UpdateDeviceRequest struct {
	DisplayName *string `json:"display_name"`
}

// DeviceResponse is the standard device representation returned by the API.
type DeviceResponse struct {
	ID             string  `json:"id"`
	GatewayID      string  `json:"gateway_id"`
	ZoneID         string  `json:"zone_id"`
	DeviceTypeCode string  `json:"device_type_code"`
	FuncTag        string  `json:"func_tag"`
	DisplayName    *string `json:"display_name"`
	DeviceCode     string  `json:"device_code"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}
