package dto

import (
	"time"

	"github.com/google/uuid"
)

// ── Shared metadata sub-structs ───────────────────────────────────────────────

type SiteBrief struct {
	ID        uuid.UUID `json:"id"`
	UtilityID string    `json:"utility_id"`
	NameCN    string    `json:"name_cn"`
}

type ZoneBrief struct {
	ID       uuid.UUID `json:"id"`
	ZoneName string    `json:"zone_name"`
}

// DeviceInfo is embedded in all single-device telemetry responses.
type DeviceInfo struct {
	DeviceID       uuid.UUID `json:"device_id"`
	DeviceTypeCode string    `json:"device_type_code"`
	FuncTag        string    `json:"func_tag"`
	DeviceCode     string    `json:"device_code"`
	Zone           ZoneBrief `json:"zone"`
	Site           SiteBrief `json:"site"`
}

// ── Single-device latest responses ───────────────────────────────────────────

type LatestMeterResponse struct {
	DeviceInfo
	TS       time.Time `json:"ts"`
	Voltage  *float64  `json:"voltage"`
	Current  *float64  `json:"current"`
	KW       *float64  `json:"kw"`
	KVA      *float64  `json:"kva"`
	KVAR     *float64  `json:"kvar"`
	KWH      *float64  `json:"kwh"`
	KVAH     *float64  `json:"kvah"`
	KVARH    *float64  `json:"kvarh"`
	CurrentA *float64  `json:"current_a"`
	CurrentB *float64  `json:"current_b"`
	CurrentC *float64  `json:"current_c"`
	PF       *float64  `json:"pf"`
	Status   *string   `json:"status"`
}

// LatestInverterResponse is the API response for inverter (CI) telemetry.
// Error and Alert are integer status codes from the inverter register.
// NULL means no code is present (device returned -1 or no data).
type LatestInverterResponse struct {
	DeviceInfo
	TS        time.Time `json:"ts"`
	Voltage   *float64  `json:"voltage"`
	Current   *float64  `json:"current"`
	KW        *float64  `json:"kw"`
	KWH       *float64  `json:"kwh"`
	HZ        *float64  `json:"hz"`
	Error     *int      `json:"error"`
	Alert     *int      `json:"alert"`
	InvStatus *string   `json:"invstatus"`
	Status    *string   `json:"status"`
}

type LatestFlowMeterResponse struct {
	DeviceInfo
	TS             time.Time `json:"ts"`
	Flow           *float64  `json:"flow"`
	Consumption    *float64  `json:"consumption"`
	RevConsumption *float64  `json:"revconsumption"`
	Direction      *int      `json:"direction"`
	Status         *string   `json:"status"`
}

type LatestSensorResponse struct {
	TS           time.Time `json:"ts"`
	AssignmentID uuid.UUID `json:"assignment_id"`
	Val          *float64  `json:"val"`
	Status       *string   `json:"status"`
}

// ── Site-wide latest response ─────────────────────────────────────────────────

// DeviceLatestEntry is one device row inside SiteLatestResponse.
// TS and Data are nil when no telemetry exists for this device yet.
type DeviceLatestEntry struct {
	DeviceID       uuid.UUID  `json:"device_id"`
	DeviceTypeCode string     `json:"device_type_code"`
	FuncTag        string     `json:"func_tag"`
	DeviceCode     string     `json:"device_code"`
	TS             *time.Time `json:"ts"`
	Data           any        `json:"data"`
}

type AssignmentLatestEntry struct {
	AssignmentID   uuid.UUID  `json:"assignment_id"`
	SensorTypeCode string     `json:"sensor_type_code"`
	FuncTag        string     `json:"func_tag"`
	SensorName     string     `json:"sensor_name"`
	Unit           string     `json:"unit"`
	TS             *time.Time `json:"ts"`
	Val            *float64   `json:"val"`
	Status         *string    `json:"status"`
}

type ZoneLatestGroup struct {
	ZoneID      uuid.UUID               `json:"zone_id"`
	ZoneName    string                  `json:"zone_name"`
	Devices     []DeviceLatestEntry     `json:"devices"`
	Assignments []AssignmentLatestEntry `json:"assignments"`
}

type SiteLatestResponse struct {
	SiteID    uuid.UUID         `json:"site_id"`
	UtilityID string            `json:"utility_id"`
	NameCN    string            `json:"name_cn"`
	Zones     []ZoneLatestGroup `json:"zones"`
}

// ── Typed data payloads used inside DeviceLatestEntry.Data ───────────────────

type MeterData struct {
	Voltage  *float64 `json:"voltage"`
	Current  *float64 `json:"current"`
	KW       *float64 `json:"kw"`
	KVA      *float64 `json:"kva"`
	KVAR     *float64 `json:"kvar"`
	KWH      *float64 `json:"kwh"`
	KVAH     *float64 `json:"kvah"`
	KVARH    *float64 `json:"kvarh"`
	CurrentA *float64 `json:"current_a"`
	CurrentB *float64 `json:"current_b"`
	CurrentC *float64 `json:"current_c"`
	PF       *float64 `json:"pf"`
	Status   *string  `json:"status"`
}

// InverterData mirrors LatestInverterResponse.Data for the site-wide snapshot.
// Error and Alert are integers (NULL = no code).
type InverterData struct {
	Voltage   *float64 `json:"voltage"`
	Current   *float64 `json:"current"`
	KW        *float64 `json:"kw"`
	KWH       *float64 `json:"kwh"`
	HZ        *float64 `json:"hz"`
	Error     *int     `json:"error"`
	Alert     *int     `json:"alert"`
	InvStatus *string  `json:"invstatus"`
	Status    *string  `json:"status"`
}

type FlowMeterData struct {
	Flow           *float64 `json:"flow"`
	Consumption    *float64 `json:"consumption"`
	RevConsumption *float64 `json:"revconsumption"`
	Direction      *int     `json:"direction"`
	Status         *string  `json:"status"`
}

// ── History query params ──────────────────────────────────────────────────────

type HistoryQuery struct {
	From time.Time `form:"from" time_format:"2006-01-02T15:04:05" time_utc:"1"`
	To   time.Time `form:"to"   time_format:"2006-01-02T15:04:05" time_utc:"1"`
}
