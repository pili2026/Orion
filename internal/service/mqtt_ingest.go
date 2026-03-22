package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/internal/model"
	"github.com/hill/orion/internal/repository"
)

const (
	dlqMaxRetries = 3
	dlqBaseDelay  = 2 * time.Second
)

const cacheTTL = 30 * time.Minute

// cacheEntry holds a resolved UUID and its expiry time.
type cacheEntry struct {
	id        uuid.UUID
	expiresAt time.Time
}

// MQTTIngestService resolves hardware identifiers to UUIDs, dispatches
// telemetry readings to the correct TimescaleDB hypertable, and retries
// failed writes via an in-memory dead-letter queue.
//
// UUID resolution is backed by an in-memory cache keyed on:
//   - "gw:{mqtt_username}"                       → gateway UUID
//   - "dev:{gateway_id}:{device_code}"            → device UUID  (SE/CI/SF)
//   - "sen:{gateway_id}:{device_code}:{pin}"      → assignment UUID (ST/SP/SR/SO)
//
// Each cache entry carries a 30-minute TTL; a background eviction worker
// removes expired entries every 5 minutes.
type MQTTIngestService struct {
	db   *gorm.DB
	repo repository.TelemetryRepository

	mu     sync.RWMutex
	cache  map[string]cacheEntry
	cancel context.CancelFunc

	dlq DLQ
	wg  sync.WaitGroup
}

func NewMQTTIngestService(db *gorm.DB, repo repository.TelemetryRepository, dlq DLQ) *MQTTIngestService {
	return &MQTTIngestService{
		db:    db,
		repo:  repo,
		cache: make(map[string]cacheEntry),
		dlq:   dlq,
	}
}

// Start launches the background DLQ retry worker and cache eviction worker.
// Must be called once after construction, before the MQTT subscriber is active.
func (s *MQTTIngestService) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(2)
	go s.dlqWorker(ctx)
	go s.cacheEvictWorker(ctx)
}

// Stop closes the DLQ channel and waits for both worker goroutines to finish.
// Call during graceful shutdown, after the MQTT client has been disconnected.
func (s *MQTTIngestService) Stop() {
	s.cancel()
	s.dlq.Close()
	s.wg.Wait()
}

// ProcessTelemetry is the primary entry point called by the MQTT handler.
// It parses the raw payload, captures the receipt timestamp, and attempts to
// write every reading.  Readings that fail UUID resolution or DB insertion are
// individually queued in the DLQ for retry.
//
// Returns a non-nil error only when the payload itself is unparseable (JSON
// syntax error, missing required fields).  Per-reading failures are handled
// internally via the DLQ and do not surface as return values.
func (s *MQTTIngestService) ProcessTelemetry(ctx context.Context, mqttUsername string, payload []byte) error {
	var p dto.TelemetryPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal telemetry payload: %w", err)
	}

	if p.TS <= 0 {
		return fmt.Errorf("invalid payload ts: %d", p.TS)
	}
	ts := time.UnixMilli(p.TS).UTC()

	receivedAt := time.Now()

	// Resolve gateway UUID — if this fails the whole payload is irrecoverable
	// until the cache/DB is updated, so queue every reading for retry.
	gwID, err := s.resolveGateway(ctx, mqttUsername)
	if err != nil {
		slog.Warn("mqtt_ingest: gateway resolution failed, queuing all readings",
			slog.String("mqtt_username", mqttUsername),
			slog.Any("error", err),
		)
		for _, r := range p.Readings {
			s.enqueueDLQ(mqttUsername, ts, receivedAt, r, 0)
		}
		return nil
	}

	for _, r := range p.Readings {
		if err := s.processReading(ctx, gwID, ts, receivedAt, r); err != nil {
			slog.Warn("mqtt_ingest: reading failed, queuing for retry",
				slog.String("type", r.Type),
				slog.String("device_code", r.DeviceCode),
				slog.Any("error", err),
			)
			s.enqueueDLQ(mqttUsername, ts, receivedAt, r, 0)
		}
	}

	return nil
}

// processReading dispatches a single reading to the correct repository Insert.
func (s *MQTTIngestService) processReading(
	ctx context.Context,
	gwID uuid.UUID,
	ts time.Time,
	receivedAt time.Time,
	r dto.TelemetryReading,
) error {
	switch r.Type {
	case model.DeviceTypeMeter:
		deviceID, err := s.resolveDevice(ctx, gwID, r.DeviceCode)
		if err != nil {
			return err
		}
		return s.repo.InsertMeter(ctx, repository.MeterRow{
			TS:       ts,
			DeviceID: deviceID,
			Voltage:  r.Voltage,
			Current:  r.Current,
			KW:       r.KW,
			KVA:      r.KVA,
			KVAR:     r.KVAR,
			KWH:      r.KWH,
			KVAH:     r.KVAH,
			KVARH:    r.KVARH,
			CurrentA: r.CurrentA,
			CurrentB: r.CurrentB,
			CurrentC: r.CurrentC,
			PF:       r.PF,
			Status:   r.Status,
		}, receivedAt)

	case model.DeviceTypeInverter:
		deviceID, err := s.resolveDevice(ctx, gwID, r.DeviceCode)
		if err != nil {
			return err
		}
		return s.repo.InsertInverter(ctx, repository.InverterRow{
			TS:        ts,
			DeviceID:  deviceID,
			Voltage:   r.Voltage,
			Current:   r.Current,
			KW:        r.KW,
			KWH:       r.KWH,
			HZ:        r.HZ,
			Error:     dto.NormalizeIntField(r.Error),
			Alert:     dto.NormalizeIntField(r.Alert),
			InvStatus: r.InvStatus,
			Status:    r.Status,
		}, receivedAt)

	case model.DeviceTypeFlow:
		deviceID, err := s.resolveDevice(ctx, gwID, r.DeviceCode)
		if err != nil {
			return err
		}
		return s.repo.InsertFlowMeter(ctx, repository.FlowMeterRow{
			TS:             ts,
			DeviceID:       deviceID,
			Flow:           r.Flow,
			Consumption:    r.Consumption,
			RevConsumption: r.RevConsumption,
			Direction:      r.Direction,
			Status:         r.Status,
		}, receivedAt)

	case model.DeviceTypeTempSensor, model.DeviceTypePressure,
		model.DeviceTypeDigital, model.DeviceTypeOxygen:
		if r.Pin == nil {
			return fmt.Errorf("sensor reading missing pin: type=%s device_code=%s", r.Type, r.DeviceCode)
		}
		assignmentID, err := s.resolveSensor(ctx, gwID, r.DeviceCode, *r.Pin)
		if err != nil {
			return err
		}
		return s.repo.InsertSensor(ctx, repository.SensorRow{
			TS:           ts,
			AssignmentID: assignmentID,
			Val:          r.Val,
			Status:       r.Status,
		}, receivedAt)

	default:
		slog.Warn("mqtt_ingest: unknown device type, skipping",
			slog.String("type", r.Type),
			slog.String("device_code", r.DeviceCode),
		)
		return nil
	}
}

// ── Cache-backed UUID resolution ──────────────────────────────────────────────

func (s *MQTTIngestService) resolveGateway(ctx context.Context, mqttUsername string) (uuid.UUID, error) {
	key := "gw:" + mqttUsername
	if id, ok := s.cacheGet(key); ok {
		return id, nil
	}

	var result struct{ ID uuid.UUID }
	if err := s.db.WithContext(ctx).
		Raw("SELECT id FROM gateways WHERE mqtt_username = ? AND deleted_at IS NULL LIMIT 1", mqttUsername).
		Scan(&result).Error; err != nil {
		return uuid.Nil, fmt.Errorf("db lookup gateway %q: %w", mqttUsername, err)
	}
	if result.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("gateway not found for mqtt_username=%q", mqttUsername)
	}
	s.cacheSet(key, result.ID)
	return result.ID, nil
}

func (s *MQTTIngestService) resolveDevice(ctx context.Context, gwID uuid.UUID, deviceCode string) (uuid.UUID, error) {
	key := "dev:" + gwID.String() + ":" + deviceCode
	if id, ok := s.cacheGet(key); ok {
		return id, nil
	}

	var result struct{ ID uuid.UUID }
	if err := s.db.WithContext(ctx).
		Raw(`SELECT id FROM devices
         WHERE gateway_id = ? AND device_code = ? AND deleted_at IS NULL LIMIT 1`,
			gwID, deviceCode).
		Scan(&result).Error; err != nil {
		return uuid.Nil, fmt.Errorf("db lookup device gw=%s code=%s: %w", gwID, deviceCode, err)
	}
	if result.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("device not found gw=%s code=%s", gwID, deviceCode)
	}
	s.cacheSet(key, result.ID)
	return result.ID, nil
}

func (s *MQTTIngestService) resolveSensor(ctx context.Context, gwID uuid.UUID, deviceCode string, pin int) (uuid.UUID, error) {
	key := "sen:" + gwID.String() + ":" + deviceCode + ":" + strconv.Itoa(pin)
	if id, ok := s.cacheGet(key); ok {
		return id, nil
	}

	var result struct{ ID uuid.UUID }
	if err := s.db.WithContext(ctx).Raw(`
    SELECT pa.id
    FROM point_assignments pa
    JOIN physical_points pp ON pa.point_id = pp.id
    JOIN devices d          ON pp.device_id = d.id
    WHERE d.gateway_id  = ?
      AND d.device_code = ?
      AND pp.port_index = ?
      AND pa.active_to  IS NULL
      AND pa.deleted_at IS NULL
    LIMIT 1
`, gwID, deviceCode, pin).Scan(&result).Error; err != nil {
		return uuid.Nil, fmt.Errorf("db lookup sensor gw=%s code=%s pin=%d: %w", gwID, deviceCode, pin, err)
	}
	if result.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("sensor assignment not found gw=%s code=%s pin=%d", gwID, deviceCode, pin)
	}
	s.cacheSet(key, result.ID)
	return result.ID, nil
}

func (s *MQTTIngestService) cacheGet(key string) (uuid.UUID, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return uuid.Nil, false
	}
	return entry.id, true
}

func (s *MQTTIngestService) cacheSet(key string, id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = cacheEntry{id: id, expiresAt: time.Now().Add(cacheTTL)}
}

// cacheEvictWorker periodically removes expired entries from the cache.
// It runs every 5 minutes and exits when ctx is cancelled.
func (s *MQTTIngestService) cacheEvictWorker(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for k, entry := range s.cache {
				if now.After(entry.expiresAt) {
					delete(s.cache, k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// ── Dead-letter queue ─────────────────────────────────────────────────────────

func (s *MQTTIngestService) enqueueDLQ(mqttUsername string, ts, receivedAt time.Time, r dto.TelemetryReading, retries int) {
	s.dlq.Enqueue(DLQMessage{
		MQTTUsername: mqttUsername,
		TS:           ts,
		ReceivedAt:   receivedAt,
		Reading:      r,
		Retries:      retries,
	})
}

// dlqWorker processes failed readings with exponential backoff.
// Delays: attempt 1 → 2 s, attempt 2 → 4 s, attempt 3 → 8 s.
// After dlqMaxRetries the reading is logged and discarded.
func (s *MQTTIngestService) dlqWorker(ctx context.Context) {
	defer s.wg.Done()
	log := slog.Default().With(slog.String("component", "MQTTIngestDLQ"))

	s.dlq.Run(ctx, func(ctx context.Context, msg DLQMessage) {
		if msg.Retries >= dlqMaxRetries {
			log.Error("DLQ: max retries exceeded, discarding reading",
				slog.String("mqtt_username", msg.MQTTUsername),
				slog.String("type", msg.Reading.Type),
				slog.String("device_code", msg.Reading.DeviceCode),
			)
			return
		}

		// Exponential backoff: 2^retries * base (2s, 4s, 8s).
		delay := dlqBaseDelay * (1 << uint(msg.Retries))
		select {
		case <-ctx.Done():
			log.Info("DLQ worker shutting down")
			return
		case <-time.After(delay):
		}

		log.Info("DLQ: retrying reading",
			slog.String("type", msg.Reading.Type),
			slog.String("device_code", msg.Reading.DeviceCode),
			slog.Int("attempt", msg.Retries+1),
		)

		gwID, err := s.resolveGateway(ctx, msg.MQTTUsername)
		if err != nil {
			log.Warn("DLQ: gateway resolution still failing",
				slog.String("mqtt_username", msg.MQTTUsername),
				slog.Any("error", err),
			)
			msg.Retries++
			s.enqueueDLQ(msg.MQTTUsername, msg.TS, msg.ReceivedAt, msg.Reading, msg.Retries)
			return
		}

		if err := s.processReading(ctx, gwID, msg.TS, msg.ReceivedAt, msg.Reading); err != nil {
			log.Warn("DLQ: retry failed",
				slog.String("type", msg.Reading.Type),
				slog.String("device_code", msg.Reading.DeviceCode),
				slog.Any("error", err),
			)
			msg.Retries++
			s.enqueueDLQ(msg.MQTTUsername, msg.TS, msg.ReceivedAt, msg.Reading, msg.Retries)
			return
		}

		log.Info("DLQ: retry succeeded",
			slog.String("type", msg.Reading.Type),
			slog.String("device_code", msg.Reading.DeviceCode),
		)
	})
}
