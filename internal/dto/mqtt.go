package dto

// TelemetryPayload is the top-level MQTT message published by a Talos Edge
// device on topic talos/{edge_id}/telemetry.
//
// All numeric fields are nullable: a nil value means the device did not
// report that measurement (e.g. sensor offline, register not supported).
type TelemetryPayload struct {
	// TS is the device-side Unix timestamp in milliseconds (UTC).
	// Example: 1742457600000 = 2026-03-20T08:00:00Z
	TS       int64              `json:"ts"`
	Readings []TelemetryReading `json:"readings"`
}

// TelemetryReading represents a single device or sensor measurement.
// The Type field determines which fields are populated.
//
// Device types:
//   - SE  — Power Meter       (telemetry_meters)
//   - CI  — Inverter          (telemetry_inverters)
//   - SF  — Flow Meter        (telemetry_flow_meters)
//   - ST  — Temperature Sensor (telemetry_sensors)
//   - SP  — Pressure Sensor   (telemetry_sensors)
//   - SR  — Digital Sensor    (telemetry_sensors)
//   - SO  — Oxygen Sensor     (telemetry_sensors)
type TelemetryReading struct {
	// Type is the device type code (SE / CI / SF / ST / SP / SR / SO).
	Type string `json:"type"`

	// DeviceCode is the Modbus address string (e.g. "1f0"),
	// matching devices.device_code in the metadata DB.
	DeviceCode string `json:"device_code"`

	// Pin is the physical_points.port_index for sensor types (ST/SP/SR/SO).
	// Required for sensors; omitted for device types (SE/CI/SF).
	Pin *int `json:"pin,omitempty"`

	// ── SE (Power Meter) fields ───────────────────────────────────────────────

	Voltage  *float64 `json:"voltage,omitempty"`
	Current  *float64 `json:"current,omitempty"`
	KW       *float64 `json:"kw,omitempty"`
	KVA      *float64 `json:"kva,omitempty"`
	KVAR     *float64 `json:"kvar,omitempty"`
	KWH      *float64 `json:"kwh,omitempty"`
	KVAH     *float64 `json:"kvah,omitempty"`
	KVARH    *float64 `json:"kvarh,omitempty"`
	CurrentA *float64 `json:"current_a,omitempty"`
	CurrentB *float64 `json:"current_b,omitempty"`
	CurrentC *float64 `json:"current_c,omitempty"`
	PF       *float64 `json:"pf,omitempty"`

	// ── CI (Inverter) additional fields ──────────────────────────────────────
	// Voltage / Current / KW / KWH are shared with SE above.

	HZ *float64 `json:"hz,omitempty"`

	// Error and Alert are integer status codes from the inverter register.
	// -1 means "no code present" and will be stored as NULL.
	Error *int `json:"error,omitempty"`
	Alert *int `json:"alert,omitempty"`

	InvStatus *string `json:"inv_status,omitempty"`

	// ── SF (Flow Meter) fields ────────────────────────────────────────────────

	Flow           *float64 `json:"flow,omitempty"`
	Consumption    *float64 `json:"consumption,omitempty"`
	RevConsumption *float64 `json:"rev_consumption,omitempty"`
	Direction      *int     `json:"direction,omitempty"`

	// ── Sensor types (ST / SP / SR / SO) ─────────────────────────────────────
	// Val is the numeric sensor value.
	// SR (digital sensor) has no numeric value; its state is packed into Status.

	Val    *float64 `json:"val,omitempty"`
	Status *string  `json:"status,omitempty"`
}

// NormalizeIntField maps -1 → nil, passing all other values through unchanged.
// Applied to CI Error and Alert fields where -1 means "register not available".
func NormalizeIntField(v *int) *int {
	if v == nil || *v == -1 {
		return nil
	}
	return v
}
