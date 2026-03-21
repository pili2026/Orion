package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hill/orion/pkg/apperr"
)

var ErrNoData = fmt.Errorf("no telemetry data found: %w", apperr.ErrNotFound)

// ── Row types ─────────────────────────────────────────────────────────────────

type MeterRow struct {
	TS       time.Time
	DeviceID uuid.UUID
	Voltage  *float64
	Current  *float64
	KW       *float64
	KVA      *float64
	KVAR     *float64
	KWH      *float64
	KVAH     *float64
	KVARH    *float64
	CurrentA *float64
	CurrentB *float64
	CurrentC *float64
	PF       *float64
	Status   *string
}

// InverterRow maps to telemetry_inverters.
// Error and Alert are *int (migration 004 converted the columns from TEXT to INTEGER).
// -1 is normalised to nil before storage; 0 means "no error / normal state".
type InverterRow struct {
	TS        time.Time
	DeviceID  uuid.UUID
	Voltage   *float64
	Current   *float64
	KW        *float64
	KWH       *float64
	HZ        *float64
	Error     *int // NULL = no error code (was -1 in legacy data)
	Alert     *int // NULL = no alert code
	InvStatus *string
	Status    *string
}

type FlowMeterRow struct {
	TS             time.Time
	DeviceID       uuid.UUID
	Flow           *float64
	Consumption    *float64
	RevConsumption *float64
	Direction      *int
	Status         *string
}

type SensorRow struct {
	TS           time.Time
	AssignmentID uuid.UUID
	Val          *float64
	Status       *string
}

// ── Interface ─────────────────────────────────────────────────────────────────

type TelemetryRepository interface {
	// ── Read path (HTTP API) ─────────────────────────────────────────────────

	// Single-entity latest
	LatestMeter(ctx context.Context, deviceID uuid.UUID) (*MeterRow, error)
	LatestInverter(ctx context.Context, deviceID uuid.UUID) (*InverterRow, error)
	LatestFlowMeter(ctx context.Context, deviceID uuid.UUID) (*FlowMeterRow, error)
	LatestSensor(ctx context.Context, assignmentID uuid.UUID) (*SensorRow, error)

	// Bulk latest (DISTINCT ON) — used by site-wide snapshot API
	LatestMetersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]MeterRow, error)
	LatestInvertersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]InverterRow, error)
	LatestFlowMetersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]FlowMeterRow, error)
	LatestSensorsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]SensorRow, error)

	// History
	MeterHistory(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]MeterRow, error)
	InverterHistory(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]InverterRow, error)
	FlowMeterHistory(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]FlowMeterRow, error)
	SensorHistory(ctx context.Context, assignmentID uuid.UUID, from, to time.Time) ([]SensorRow, error)

	// ── Write path (MQTT ingest) ─────────────────────────────────────────────

	// receivedAt is the Orion-side wall-clock time (time.Now() at MQTT receipt).
	// It is stored in the received_at column alongside ts (device-side time)
	// so that network latency and device clock drift can be monitored.
	InsertMeter(ctx context.Context, row MeterRow, receivedAt time.Time) error
	InsertInverter(ctx context.Context, row InverterRow, receivedAt time.Time) error
	InsertFlowMeter(ctx context.Context, row FlowMeterRow, receivedAt time.Time) error
	InsertSensor(ctx context.Context, row SensorRow, receivedAt time.Time) error
}

type telemetryRepository struct {
	pool *pgxpool.Pool
}

func NewTelemetryRepository(pool *pgxpool.Pool) TelemetryRepository {
	return &telemetryRepository{pool: pool}
}

// ── Single latest ─────────────────────────────────────────────────────────────

func (r *telemetryRepository) LatestMeter(ctx context.Context, deviceID uuid.UUID) (*MeterRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT ts, device_id, voltage, current, kw, kva, kvar,
		       kwh, kvah, kvarh, current_a, current_b, current_c, pf, status
		FROM telemetry_meters
		WHERE device_id = $1
		ORDER BY ts DESC LIMIT 1
	`, deviceID)
	var m MeterRow
	err := row.Scan(
		&m.TS, &m.DeviceID,
		&m.Voltage, &m.Current, &m.KW, &m.KVA, &m.KVAR,
		&m.KWH, &m.KVAH, &m.KVARH,
		&m.CurrentA, &m.CurrentB, &m.CurrentC,
		&m.PF, &m.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoData
	}
	if err != nil {
		return nil, fmt.Errorf("latest meter: %w", err)
	}
	return &m, nil
}

func (r *telemetryRepository) LatestInverter(ctx context.Context, deviceID uuid.UUID) (*InverterRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT ts, device_id, voltage, current, kw, kwh,
		       hz, error, alert, invstatus, status
		FROM telemetry_inverters
		WHERE device_id = $1
		ORDER BY ts DESC LIMIT 1
	`, deviceID)
	var m InverterRow
	err := row.Scan(
		&m.TS, &m.DeviceID,
		&m.Voltage, &m.Current, &m.KW, &m.KWH,
		&m.HZ, &m.Error, &m.Alert, &m.InvStatus, &m.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoData
	}
	if err != nil {
		return nil, fmt.Errorf("latest inverter: %w", err)
	}
	return &m, nil
}

func (r *telemetryRepository) LatestFlowMeter(ctx context.Context, deviceID uuid.UUID) (*FlowMeterRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT ts, device_id, flow, consumption, revconsumption, direction, status
		FROM telemetry_flow_meters
		WHERE device_id = $1
		ORDER BY ts DESC LIMIT 1
	`, deviceID)
	var m FlowMeterRow
	err := row.Scan(
		&m.TS, &m.DeviceID,
		&m.Flow, &m.Consumption, &m.RevConsumption, &m.Direction, &m.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoData
	}
	if err != nil {
		return nil, fmt.Errorf("latest flow meter: %w", err)
	}
	return &m, nil
}

func (r *telemetryRepository) LatestSensor(ctx context.Context, assignmentID uuid.UUID) (*SensorRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT ts, assignment_id, val, status
		FROM telemetry_sensors
		WHERE assignment_id = $1
		ORDER BY ts DESC LIMIT 1
	`, assignmentID)
	var m SensorRow
	err := row.Scan(&m.TS, &m.AssignmentID, &m.Val, &m.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoData
	}
	if err != nil {
		return nil, fmt.Errorf("latest sensor: %w", err)
	}
	return &m, nil
}

// ── Bulk latest (DISTINCT ON) ─────────────────────────────────────────────────

func (r *telemetryRepository) LatestMetersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]MeterRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (device_id)
		       ts, device_id, voltage, current, kw, kva, kvar,
		       kwh, kvah, kvarh, current_a, current_b, current_c, pf, status
		FROM telemetry_meters
		WHERE device_id = ANY($1)
		ORDER BY device_id, ts DESC
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("latest meters by ids: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]MeterRow, len(ids))
	for rows.Next() {
		var m MeterRow
		if err := rows.Scan(
			&m.TS, &m.DeviceID,
			&m.Voltage, &m.Current, &m.KW, &m.KVA, &m.KVAR,
			&m.KWH, &m.KVAH, &m.KVARH,
			&m.CurrentA, &m.CurrentB, &m.CurrentC,
			&m.PF, &m.Status,
		); err != nil {
			return nil, fmt.Errorf("scan meter row: %w", err)
		}
		result[m.DeviceID] = m
	}
	return result, rows.Err()
}

func (r *telemetryRepository) LatestInvertersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]InverterRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (device_id)
		       ts, device_id, voltage, current, kw, kwh,
		       hz, error, alert, invstatus, status
		FROM telemetry_inverters
		WHERE device_id = ANY($1)
		ORDER BY device_id, ts DESC
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("latest inverters by ids: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]InverterRow, len(ids))
	for rows.Next() {
		var m InverterRow
		if err := rows.Scan(
			&m.TS, &m.DeviceID,
			&m.Voltage, &m.Current, &m.KW, &m.KWH,
			&m.HZ, &m.Error, &m.Alert, &m.InvStatus, &m.Status,
		); err != nil {
			return nil, fmt.Errorf("scan inverter row: %w", err)
		}
		result[m.DeviceID] = m
	}
	return result, rows.Err()
}

func (r *telemetryRepository) LatestFlowMetersByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]FlowMeterRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (device_id)
		       ts, device_id, flow, consumption, revconsumption, direction, status
		FROM telemetry_flow_meters
		WHERE device_id = ANY($1)
		ORDER BY device_id, ts DESC
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("latest flow meters by ids: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]FlowMeterRow, len(ids))
	for rows.Next() {
		var m FlowMeterRow
		if err := rows.Scan(
			&m.TS, &m.DeviceID,
			&m.Flow, &m.Consumption, &m.RevConsumption, &m.Direction, &m.Status,
		); err != nil {
			return nil, fmt.Errorf("scan flow meter row: %w", err)
		}
		result[m.DeviceID] = m
	}
	return result, rows.Err()
}

func (r *telemetryRepository) LatestSensorsByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]SensorRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (assignment_id)
		       ts, assignment_id, val, status
		FROM telemetry_sensors
		WHERE assignment_id = ANY($1)
		ORDER BY assignment_id, ts DESC
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("latest sensors by ids: %w", err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID]SensorRow, len(ids))
	for rows.Next() {
		var m SensorRow
		if err := rows.Scan(&m.TS, &m.AssignmentID, &m.Val, &m.Status); err != nil {
			return nil, fmt.Errorf("scan sensor row: %w", err)
		}
		result[m.AssignmentID] = m
	}
	return result, rows.Err()
}

// ── History ───────────────────────────────────────────────────────────────────

func (r *telemetryRepository) MeterHistory(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]MeterRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ts, device_id, voltage, current, kw, kva, kvar,
		       kwh, kvah, kvarh, current_a, current_b, current_c, pf, status
		FROM telemetry_meters
		WHERE device_id = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts ASC
	`, deviceID, from, to)
	if err != nil {
		return nil, fmt.Errorf("meter history: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (MeterRow, error) {
		var m MeterRow
		err := row.Scan(
			&m.TS, &m.DeviceID,
			&m.Voltage, &m.Current, &m.KW, &m.KVA, &m.KVAR,
			&m.KWH, &m.KVAH, &m.KVARH,
			&m.CurrentA, &m.CurrentB, &m.CurrentC,
			&m.PF, &m.Status,
		)
		return m, err
	})
}

func (r *telemetryRepository) InverterHistory(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]InverterRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ts, device_id, voltage, current, kw, kwh,
		       hz, error, alert, invstatus, status
		FROM telemetry_inverters
		WHERE device_id = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts ASC
	`, deviceID, from, to)
	if err != nil {
		return nil, fmt.Errorf("inverter history: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (InverterRow, error) {
		var m InverterRow
		err := row.Scan(
			&m.TS, &m.DeviceID,
			&m.Voltage, &m.Current, &m.KW, &m.KWH,
			&m.HZ, &m.Error, &m.Alert, &m.InvStatus, &m.Status,
		)
		return m, err
	})
}

func (r *telemetryRepository) FlowMeterHistory(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]FlowMeterRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ts, device_id, flow, consumption, revconsumption, direction, status
		FROM telemetry_flow_meters
		WHERE device_id = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts ASC
	`, deviceID, from, to)
	if err != nil {
		return nil, fmt.Errorf("flow meter history: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (FlowMeterRow, error) {
		var m FlowMeterRow
		err := row.Scan(
			&m.TS, &m.DeviceID,
			&m.Flow, &m.Consumption, &m.RevConsumption, &m.Direction, &m.Status,
		)
		return m, err
	})
}

func (r *telemetryRepository) SensorHistory(ctx context.Context, assignmentID uuid.UUID, from, to time.Time) ([]SensorRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ts, assignment_id, val, status
		FROM telemetry_sensors
		WHERE assignment_id = $1 AND ts >= $2 AND ts <= $3
		ORDER BY ts ASC
	`, assignmentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("sensor history: %w", err)
	}
	defer rows.Close()
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SensorRow, error) {
		var m SensorRow
		err := row.Scan(&m.TS, &m.AssignmentID, &m.Val, &m.Status)
		return m, err
	})
}

// ── Write path (MQTT ingest) ──────────────────────────────────────────────────

func (r *telemetryRepository) InsertMeter(ctx context.Context, row MeterRow, receivedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO telemetry_meters
		    (ts, device_id, voltage, current, kw, kva, kvar,
		     kwh, kvah, kvarh, current_a, current_b, current_c, pf, status, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
	`,
		row.TS, row.DeviceID,
		row.Voltage, row.Current, row.KW, row.KVA, row.KVAR,
		row.KWH, row.KVAH, row.KVARH,
		row.CurrentA, row.CurrentB, row.CurrentC,
		row.PF, row.Status,
		receivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert meter: %w", err)
	}
	return nil
}

func (r *telemetryRepository) InsertInverter(ctx context.Context, row InverterRow, receivedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO telemetry_inverters
		    (ts, device_id, voltage, current, kw, kwh,
		     hz, error, alert, invstatus, status, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`,
		row.TS, row.DeviceID,
		row.Voltage, row.Current, row.KW, row.KWH,
		row.HZ, row.Error, row.Alert, row.InvStatus, row.Status,
		receivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert inverter: %w", err)
	}
	return nil
}

func (r *telemetryRepository) InsertFlowMeter(ctx context.Context, row FlowMeterRow, receivedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO telemetry_flow_meters
		    (ts, device_id, flow, consumption, revconsumption, direction, status, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		row.TS, row.DeviceID,
		row.Flow, row.Consumption, row.RevConsumption, row.Direction, row.Status,
		receivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert flow meter: %w", err)
	}
	return nil
}

func (r *telemetryRepository) InsertSensor(ctx context.Context, row SensorRow, receivedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO telemetry_sensors
		    (ts, assignment_id, val, status, received_at)
		VALUES ($1,$2,$3,$4,$5)
	`,
		row.TS, row.AssignmentID, row.Val, row.Status,
		receivedAt,
	)
	if err != nil {
		return fmt.Errorf("insert sensor: %w", err)
	}
	return nil
}
