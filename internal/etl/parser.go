// Package etl provides utilities for migrating data from the legacy ima_thing
// database into the Orion schema.
package etl

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsedTable holds the structured information extracted from a legacy table name.
//
// Legacy format: {utility_id}_{loop}{slave}{pin}{type}
// Example:       04828405156_1f0sf
//
//	utility_id = "04828405156"
//	loop       = 1
//	slave      = "f"  (alphanumeric, NOT necessarily valid hex)
//	pin        = 0
//	DeviceType = "sf"
//
// NOTE: The suffix parser currently assumes 1-digit loop, 1-alphanumeric-char slave,
// 1-digit pin, and 2-char device type (total 5 chars).
// When multi-character slave identifiers are introduced, only parseSuffix() needs updating.
type ParsedTable struct {
	OldTableName string
	UtilityID    string
	Loop         int
	Slave        string // raw alphanumeric slave identifier as it appears in the table name (e.g. "f", "m")
	Pin          int
	DeviceType   string // lowercase, e.g. "sf", "se", "ci"
}

// IsTelemetry returns true for device types that produce time-series data
// that should be migrated to a telemetry hypertable.
func (p *ParsedTable) IsTelemetry() bool {
	switch strings.ToLower(p.DeviceType) {
	case "ci", "se", "sf", "st", "sp", "sr", "so":
		return true
	}
	return false
}

// IsSensor returns true for types that map to telemetry_sensors
// (identified by assignment_id rather than device_id).
func (p *ParsedTable) IsSensor() bool {
	switch strings.ToLower(p.DeviceType) {
	case "st", "sp", "sr", "so":
		return true
	}
	return false
}

// TelemetryTable returns the target Orion hypertable name for this device type.
func (p *ParsedTable) TelemetryTable() string {
	switch strings.ToLower(p.DeviceType) {
	case "se":
		return "telemetry_meters"
	case "ci":
		return "telemetry_inverters"
	case "sf":
		return "telemetry_flow_meters"
	case "st", "sp", "sr", "so":
		return "telemetry_sensors"
	}
	return ""
}

// GatewayKey returns a stable string that uniquely identifies the gateway
// this device belongs to: "{utility_id}-loop{loop}".
func (p *ParsedTable) GatewayKey() string {
	return fmt.Sprintf("%s-loop%d", p.UtilityID, p.Loop)
}

// ParseTableName parses a legacy ima_thing table name into its components.
// It returns an error for tables that do not match the expected format
// (e.g. internal Postgres system tables).
func ParseTableName(tableName string) (*ParsedTable, error) {
	// Split on the first underscore to separate utility_id from the device suffix.
	idx := strings.Index(tableName, "_")
	if idx < 0 {
		return nil, fmt.Errorf("no underscore in table name %q", tableName)
	}

	utilityID := tableName[:idx]
	suffix := tableName[idx+1:]

	parsed, err := parseSuffix(suffix)
	if err != nil {
		return nil, fmt.Errorf("parse suffix %q of table %q: %w", suffix, tableName, err)
	}

	parsed.OldTableName = tableName
	parsed.UtilityID = utilityID
	return parsed, nil
}

// parseSuffix parses the part after the underscore.
//
// Current format (5 chars): {1-loop}{1-alnum-slave}{1-pin}{2-type}
//
// The slave field is treated as a raw alphanumeric string — no numeric conversion
// is performed. Legacy data contains non-hex characters (e.g. "m") that would
// cause strconv.ParseInt(s, 16, 64) to fail.
//
// TODO(multi-char-slave): The slave field is currently assumed to be exactly
// 1 character (suffix[1:2]). If future device hardware introduces multi-character
// slave identifiers, adjust slaveEnd below and update device_code generation in
// seeder.go accordingly. The split point between slave and pin depends on this
// constant, so changing it here is the single place that needs updating.
func parseSuffix(suffix string) (*ParsedTable, error) {
	// Minimum length: 1 (loop) + 1 (slave) + 1 (pin) + 2 (type) = 5
	if len(suffix) < 5 {
		return nil, fmt.Errorf("suffix %q too short (expected >= 5 chars)", suffix)
	}

	// ── loop (1 digit, decimal) ───────────────────────────────────────────────
	loop, err := strconv.Atoi(string(suffix[0]))
	if err != nil {
		return nil, fmt.Errorf("loop char %q is not a digit: %w", string(suffix[0]), err)
	}

	// ── slave (1 alphanumeric char) ───────────────────────────────────────────
	// Stored as-is without any numeric conversion. Legacy tables use characters
	// outside the hex range (0-9, a-f), so no ParseInt is applied here.
	// TODO(multi-char-slave): Change slaveEnd to widen the slave field if needed.
	const slaveEnd = 2 // exclusive index: suffix[1:slaveEnd]
	slave := suffix[1:slaveEnd]

	// ── pin (1 digit, decimal) ────────────────────────────────────────────────
	pin, err := strconv.Atoi(string(suffix[slaveEnd]))
	if err != nil {
		return nil, fmt.Errorf("pin char %q is not a digit: %w", string(suffix[slaveEnd]), err)
	}

	// ── device type (remaining chars, lowercase) ──────────────────────────────
	deviceType := strings.ToLower(suffix[slaveEnd+1:])
	if deviceType == "" {
		return nil, fmt.Errorf("missing device type in suffix %q", suffix)
	}

	return &ParsedTable{
		Loop:       loop,
		Slave:      slave,
		Pin:        pin,
		DeviceType: deviceType,
	}, nil
}
