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
// Legacy format: {utility_id}_{loop}{slave_hex}{pin}{type}
// Example:       04828405156_1f0sf
//
//	utility_id = "04828405156"
//	loop       = 1
//	slave_hex  = "f"  → SlaveID = 15
//	pin        = 0
//	DeviceType = "sf"
//
// NOTE: The suffix parser currently assumes 1-digit loop, 1-digit hex slave,
// 1-digit pin, and 2-char device type (total 5 chars).
// When multi-digit slave IDs are introduced, only parseSuffix() needs updating.
type ParsedTable struct {
	OldTableName string
	UtilityID    string
	Loop         int
	SlaveID      int    // decimal representation of the hex slave byte
	SlaveHex     string // original hex string (e.g. "f")
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
// Current format (5 chars): {1-loop}{1-hex-slave}{1-pin}{2-type}
//
// To support multi-digit slave IDs in the future, update the slaveEnd index
// and adjust the pin/type extraction accordingly. The rest of the ETL pipeline
// does not need to change.
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

	// ── slave_id (1 hex digit) ────────────────────────────────────────────────
	// FUTURE: change slaveEnd to 3 or 4 to support 2- or 3-digit hex slaves.
	const slaveEnd = 2 // exclusive index: suffix[1:slaveEnd]
	slaveHex := suffix[1:slaveEnd]
	slaveVal, err := strconv.ParseInt(slaveHex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("slave hex %q is not valid hex: %w", slaveHex, err)
	}

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
		SlaveID:    int(slaveVal),
		SlaveHex:   slaveHex,
		Pin:        pin,
		DeviceType: deviceType,
	}, nil
}
