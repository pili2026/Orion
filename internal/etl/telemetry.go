// Package etl provides tools for migrating legacy ima_thing data into Orion.
package etl

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

const batchSize = 1000

// ── TelemetryConfig ───────────────────────────────────────────────────────────

// TelemetryConfig controls ETL behaviour and source DB protection.
type TelemetryConfig struct {
	Workers      int       // number of concurrent table workers
	BatchSleepMs int       // ms to sleep between batches (rate limiting)
	From         time.Time // only migrate rows with id >= From
	To           time.Time // only migrate rows with id <= To
}

// ── ETLCheckpoint ─────────────────────────────────────────────────────────────

// ETLCheckpoint tracks per-table migration progress for checkpoint/resume.
type ETLCheckpoint struct {
	OldTableName string     `gorm:"primaryKey;type:varchar(200)"`
	LastID       *time.Time `gorm:"type:timestamptz"`
	Status       string     `gorm:"type:varchar(50);default:'pending'"`
	RowsMigrated int64      `gorm:"default:0"`
	UpdatedAt    time.Time
}

func (ETLCheckpoint) TableName() string { return "etl_checkpoints" }

// ── TelemetryWorker ───────────────────────────────────────────────────────────

// TelemetryWorker migrates historical time-series data from legacy MariaDB
// tables into the Orion TimescaleDB hypertables.
//
// It reads all entries from etl_table_map, spawns a configurable number of
// concurrent workers, and processes each table in batches.  Each batch is
// committed with a checkpoint update so that interrupted runs can be resumed.
type TelemetryWorker struct {
	srcDB   *gorm.DB      // legacy MariaDB
	dstPool *pgxpool.Pool // fast CopyFrom
	dstDB   *gorm.DB      // checkpoint management (GORM)
	cfg     TelemetryConfig
}

// NewTelemetryWorker creates a TelemetryWorker.
func NewTelemetryWorker(srcDB *gorm.DB, dstPool *pgxpool.Pool, dstDB *gorm.DB, cfg TelemetryConfig) *TelemetryWorker {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.BatchSleepMs < 0 {
		cfg.BatchSleepMs = 0
	}
	return &TelemetryWorker{
		srcDB:   srcDB,
		dstPool: dstPool,
		dstDB:   dstDB,
		cfg:     cfg,
	}
}

// Run loads the full etl_table_map and migrates every table with a worker pool.
func (w *TelemetryWorker) Run(ctx context.Context) error {
	log := slog.Default().With(slog.String("component", "TelemetryWorker"))

	var entries []ETLTableMap
	if err := w.dstDB.WithContext(ctx).Find(&entries).Error; err != nil {
		return fmt.Errorf("load etl_table_map: %w", err)
	}
	log.Info("Loaded table map", slog.Int("tables", len(entries)))

	sem := make(chan struct{}, w.cfg.Workers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failedTables []string

	for _, entry := range entries {
		e := entry
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := w.processTable(ctx, e); err != nil {
				mu.Lock()
				failedTables = append(failedTables, fmt.Sprintf("%s: %v", e.OldTableName, err))
				mu.Unlock()
				log.Error("Table migration failed",
					slog.String("table", e.OldTableName),
					slog.Any("error", err),
				)
			}
		}()
	}
	wg.Wait()

	if len(failedTables) > 0 {
		log.Warn("Some tables failed", slog.Int("count", len(failedTables)))
		return fmt.Errorf("%d table(s) failed:\n%s", len(failedTables), strings.Join(failedTables, "\n"))
	}
	return nil
}

// processTable migrates one legacy table in paginated batches.
func (w *TelemetryWorker) processTable(ctx context.Context, entry ETLTableMap) error {
	log := slog.Default().With(
		slog.String("component", "TelemetryWorker"),
		slog.String("table", entry.OldTableName),
		slog.String("type", strings.ToUpper(entry.DeviceType)),
	)

	// ── Load or create checkpoint ─────────────────────────────────────────
	// Checkpoint key includes the date range so changing ETL_FROM/ETL_TO
	// starts a fresh run rather than resuming an unrelated one.
	cpKey := fmt.Sprintf("%s|%s|%s",
		entry.OldTableName,
		w.cfg.From.Format("20060102"),
		w.cfg.To.Format("20060102"),
	)
	cp := &ETLCheckpoint{OldTableName: cpKey}
	w.dstDB.WithContext(ctx).
		Where(ETLCheckpoint{OldTableName: cpKey}).
		FirstOrCreate(cp)

	if cp.Status == "done" {
		log.Info("Already completed, skipping")
		return nil
	}

	cp.Status = "running"
	w.dstDB.WithContext(ctx).Save(cp)

	sqlDB, err := w.srcDB.DB()
	if err != nil {
		return fmt.Errorf("get raw sql.DB: %w", err)
	}

	// ── Process in batches ────────────────────────────────────────────────
	for {
		rows, maxTS, err := w.fetchAndTransform(ctx, sqlDB, entry, cp.LastID, w.cfg.From, w.cfg.To)
		if err != nil {
			// Table does not exist in this source DB — skip silently.
			if isTableNotFound(err) {
				log.Info("Table not found in source DB, skipping")
				cp.Status = "skipped"
				w.dstDB.WithContext(ctx).Save(cp)
				return nil
			}
			return fmt.Errorf("fetch batch: %w", err)
		}
		if len(rows) == 0 {
			break
		}

		if err := w.copyToDest(ctx, entry, rows); err != nil {
			return fmt.Errorf("copy to dest: %w", err)
		}

		cp.LastID = &maxTS
		cp.RowsMigrated += int64(len(rows))
		w.dstDB.WithContext(ctx).Save(cp)

		log.Info("Batch written",
			slog.Int("rows", len(rows)),
			slog.Int64("total", cp.RowsMigrated),
			slog.Time("last_id", maxTS),
		)

		// Rate-limit: protect the source DB from being overwhelmed.
		if w.cfg.BatchSleepMs > 0 {
			time.Sleep(time.Duration(w.cfg.BatchSleepMs) * time.Millisecond)
		}

		if len(rows) < batchSize {
			break
		}
	}

	cp.Status = "done"
	w.dstDB.WithContext(ctx).Save(cp)
	log.Info("Completed", slog.Int64("total_rows", cp.RowsMigrated))
	return nil
}

// ── fetchAndTransform ─────────────────────────────────────────────────────────

// fetchAndTransform reads one batch from MariaDB and transforms each row into
// the []any slice expected by pgx.CopyFrom.
// Returns the transformed rows and the maximum timestamp in the batch.
func (w *TelemetryWorker) fetchAndTransform(
	ctx context.Context,
	sqlDB *sql.DB,
	entry ETLTableMap,
	after *time.Time,
	from time.Time,
	to time.Time,
) ([][]any, time.Time, error) {
	dt := strings.ToLower(entry.DeviceType)

	// Resume from checkpoint, but never go earlier than cfg.From.
	since := from
	if after != nil && after.After(from) {
		since = *after
	}

	query := buildQuery(entry.OldTableName, dt)
	rows, err := sqlDB.QueryContext(ctx, query, since, to, batchSize)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var result [][]any
	var maxTS time.Time

	switch dt {
	case "se":
		result, maxTS, err = scanSE(rows, entry)
	case "ci":
		result, maxTS, err = scanCI(rows, entry)
	case "sf":
		result, maxTS, err = scanSF(rows, entry)
	case "st", "sp", "sr", "so":
		result, maxTS, err = scanSensor(rows, entry, dt)
	default:
		return nil, time.Time{}, fmt.Errorf("unknown device type: %s", dt)
	}

	return result, maxTS, err
}

// buildQuery returns the paginated SELECT for each device type.
// Parameters: since (lower bound, exclusive), to (upper bound, inclusive), limit.
func buildQuery(tableName, dt string) string {
	q := "SELECT %s FROM `" + tableName + "` WHERE id > ? AND id <= ? ORDER BY id LIMIT ?"
	switch dt {
	case "se":
		return fmt.Sprintf(q, "id,voltage,current,kw,kva,kvar,kwh,kvah,kvarh,current_A,current_B,current_C,pf,status")
	case "ci":
		return fmt.Sprintf(q, "id,voltage,current,kw,kwh,hz,error,alert,invstatus,status")
	case "sf":
		return fmt.Sprintf(q, "id,flow,consumption,revconsumption,direction")
	default: // st, sp, sr, so — handled by name at transform time
		return fmt.Sprintf(q, "*")
	}
}

// ── per-type scan helpers ─────────────────────────────────────────────────────

func scanSE(rows *sql.Rows, entry ETLTableMap) ([][]any, time.Time, error) {
	var out [][]any
	var maxTS time.Time
	for rows.Next() {
		var (
			ts                               time.Time
			voltage, current, kw, kva, kvar  sql.NullFloat64
			kwh, kvah, kvarh                 sql.NullFloat64
			currentA, currentB, currentC, pf sql.NullFloat64
			status                           sql.NullString
		)
		if err := rows.Scan(&ts, &voltage, &current, &kw, &kva, &kvar,
			&kwh, &kvah, &kvarh, &currentA, &currentB, &currentC, &pf, &status); err != nil {
			return nil, maxTS, err
		}
		if ts.After(maxTS) {
			maxTS = ts
		}
		out = append(out, []any{
			ts, *entry.DeviceID,
			nullFloat(voltage), nullFloat(current),
			nullFloat(kw), nullFloat(kva), nullFloat(kvar),
			nullFloat(kwh), nullFloat(kvah), nullFloat(kvarh),
			nullFloat(currentA), nullFloat(currentB), nullFloat(currentC),
			nullFloat(pf), nullStr(status),
		})
	}
	return out, maxTS, rows.Err()
}

func scanCI(rows *sql.Rows, entry ETLTableMap) ([][]any, time.Time, error) {
	var out [][]any
	var maxTS time.Time
	for rows.Next() {
		var (
			ts                        time.Time
			voltage, current, kw, kwh sql.NullFloat64
			hz                        sql.NullFloat64
			errCode, alert            sql.NullString
			invstatus                 []byte
			status                    sql.NullString
		)
		if err := rows.Scan(&ts, &voltage, &current, &kw, &kwh,
			&hz, &errCode, &alert, &invstatus, &status); err != nil {
			return nil, maxTS, err
		}
		if ts.After(maxTS) {
			maxTS = ts
		}
		// invstatus binary(1) → hex string
		var invstatusStr *string
		if len(invstatus) > 0 {
			s := hex.EncodeToString(invstatus)
			invstatusStr = &s
		}
		out = append(out, []any{
			ts, *entry.DeviceID,
			nullFloat(voltage), nullFloat(current),
			nullFloat(kw), nullFloat(kwh),
			nullFloat(hz), nullStr(errCode), nullStr(alert),
			invstatusStr, nullStr(status),
		})
	}
	return out, maxTS, rows.Err()
}

func scanSF(rows *sql.Rows, entry ETLTableMap) ([][]any, time.Time, error) {
	var out [][]any
	var maxTS time.Time
	for rows.Next() {
		var (
			ts                                time.Time
			flow, consumption, revconsumption sql.NullFloat64
			directionRaw                      sql.NullString
		)
		if err := rows.Scan(&ts, &flow, &consumption, &revconsumption, &directionRaw); err != nil {
			return nil, maxTS, err
		}
		if ts.After(maxTS) {
			maxTS = ts
		}
		// direction varchar(10) → int (default 0 if unparseable)
		var direction *int
		if directionRaw.Valid {
			if n, err := strconv.Atoi(strings.TrimSpace(directionRaw.String)); err == nil {
				direction = &n
			} else {
				zero := 0
				direction = &zero
			}
		}
		out = append(out, []any{
			ts, *entry.DeviceID,
			nullFloat(flow), nullFloat(consumption), nullFloat(revconsumption),
			direction, nil, // status not present in source
		})
	}
	return out, maxTS, rows.Err()
}

func scanSensor(rows *sql.Rows, entry ETLTableMap, dt string) ([][]any, time.Time, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, time.Time{}, err
	}

	var out [][]any
	var maxTS time.Time

	for rows.Next() {
		// Generic scan into interface slice.
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, maxTS, err
		}

		colMap := make(map[string]any, len(cols))
		for i, c := range cols {
			colMap[strings.ToLower(c)] = vals[i]
		}

		ts, ok := colMap["id"].(time.Time)
		if !ok {
			continue
		}
		if ts.After(maxTS) {
			maxTS = ts
		}

		val, status := extractSensorValStatus(colMap, dt)
		out = append(out, []any{ts, *entry.AssignmentID, val, status})
	}
	return out, maxTS, rows.Err()
}

// extractSensorValStatus maps sensor column values to (val, status) pair.
func extractSensorValStatus(colMap map[string]any, dt string) (*float64, *string) {
	switch dt {
	case "st":
		return colFloat(colMap, "temperature"), colString(colMap, "status")
	case "sp":
		return colFloat(colMap, "pressure"), colString(colMap, "status")
	case "sr":
		// SR has no direct numeric value; pack all fields into status string.
		parts := []string{}
		for _, k := range []string{"ctrl", "state", "bypass", "status"} {
			if v := colString(colMap, k); v != nil {
				parts = append(parts, k+":"+*v)
			}
		}
		var s *string
		if len(parts) > 0 {
			joined := strings.Join(parts, ",")
			s = &joined
		}
		return nil, s
	case "so":
		// SO likely has an oxygen/analog value; try common column names.
		for _, name := range []string{"oxygen", "o2", "val", "value"} {
			if v := colFloat(colMap, name); v != nil {
				return v, colString(colMap, "status")
			}
		}
		return nil, colString(colMap, "status")
	}
	return nil, nil
}

// ── copyToDest ────────────────────────────────────────────────────────────────

// destTable returns the TimescaleDB hypertable name for a device type.
var destTableCols = map[string]struct {
	table string
	cols  []string
}{
	"se": {
		"telemetry_meters",
		[]string{"ts", "device_id", "voltage", "current", "kw", "kva", "kvar",
			"kwh", "kvah", "kvarh", "current_a", "current_b", "current_c", "pf", "status"},
	},
	"ci": {
		"telemetry_inverters",
		[]string{"ts", "device_id", "voltage", "current", "kw", "kwh",
			"hz", "error", "alert", "invstatus", "status"},
	},
	"sf": {
		"telemetry_flow_meters",
		[]string{"ts", "device_id", "flow", "consumption", "revconsumption", "direction", "status"},
	},
	"st": {"telemetry_sensors", []string{"ts", "assignment_id", "val", "status"}},
	"sp": {"telemetry_sensors", []string{"ts", "assignment_id", "val", "status"}},
	"sr": {"telemetry_sensors", []string{"ts", "assignment_id", "val", "status"}},
	"so": {"telemetry_sensors", []string{"ts", "assignment_id", "val", "status"}},
}

func (w *TelemetryWorker) copyToDest(ctx context.Context, entry ETLTableMap, rows [][]any) error {
	dt := strings.ToLower(entry.DeviceType)
	meta, ok := destTableCols[dt]
	if !ok {
		return fmt.Errorf("no dest table mapping for type %q", dt)
	}

	_, err := w.dstPool.CopyFrom(
		ctx,
		pgx.Identifier{meta.table},
		meta.cols,
		pgx.CopyFromRows(rows),
	)
	return err
}

// ── value helpers ─────────────────────────────────────────────────────────────

func nullFloat(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	return &n.Float64
}

func nullStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	return &n.String
}

func colFloat(m map[string]any, key string) *float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		return &val
	case []byte:
		f, err := strconv.ParseFloat(string(val), 64)
		if err != nil {
			return nil
		}
		return &f
	}
	return nil
}

func colString(m map[string]any, key string) *string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return &val
	case []byte:
		s := strings.TrimSpace(string(val))
		if s == "" {
			return nil
		}
		return &s
	}
	return nil
}

// isTableNotFound checks whether the error indicates a missing table in MySQL.
func isTableNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "table") && strings.Contains(msg, "not found")
}
