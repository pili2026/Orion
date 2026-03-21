package model

const (
	DeviceTypeMeter      = "SE"
	DeviceTypeInverter   = "CI"
	DeviceTypeFlow       = "SF"
	DeviceTypeTempSensor = "ST"
	DeviceTypePressure   = "SP"
	DeviceTypeDigital    = "SR"
	DeviceTypeOxygen     = "SO"
	DeviceTypeGateway    = "GW"
)

// IsSensor returns true if the type writes to telemetry_sensors.
func IsSensor(code string) bool {
	switch code {
	case DeviceTypeTempSensor, DeviceTypePressure, DeviceTypeDigital, DeviceTypeOxygen:
		return true
	}
	return false
}
