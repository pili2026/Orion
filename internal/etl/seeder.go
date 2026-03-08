package etl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hill/orion/internal/model"
)

// TableRow is a raw row from information_schema.tables.
type TableRow struct {
	TableName string
}

// ETLTableMap is a GORM model for the etl_table_map table.
type ETLTableMap struct {
	OldTableName string     `gorm:"primaryKey;type:varchar(200)"`
	UtilityID    string     `gorm:"type:varchar(100);not null"`
	DeviceType   string     `gorm:"type:varchar(10);not null"`
	DeviceID     *uuid.UUID `gorm:"type:uuid"`
	AssignmentID *uuid.UUID `gorm:"type:uuid"`
}

func (ETLTableMap) TableName() string { return "etl_table_map" }

// MetaSeeder scans the legacy ima_thing database and seeds the Orion metadata tables.
type MetaSeeder struct {
	// srcDB is a GORM connection to the legacy ima_thing database.
	srcDB *gorm.DB
	// dstDB is a GORM connection to the Orion database.
	dstDB *gorm.DB
}

// NewMetaSeeder creates a new MetaSeeder.
func NewMetaSeeder(srcDB, dstDB *gorm.DB) *MetaSeeder {
	return &MetaSeeder{srcDB: srcDB, dstDB: dstDB}
}

// Run performs the full metadata seeding process:
//  1. Lists all tables in the legacy schema.
//  2. Parses each table name.
//  3. Creates Sites, Gateways, and Devices in Orion (idempotent upserts).
//  4. Writes the old_table_name → device/assignment UUID mapping to etl_table_map.
func (s *MetaSeeder) Run(ctx context.Context, srcSchema string) error {
	log := slog.Default().With(slog.String("component", "MetaSeeder"))

	// ── 1. List all tables in the legacy schema ───────────────────────────────
	tables, err := s.listTables(ctx, srcSchema)
	if err != nil {
		return fmt.Errorf("list tables in schema %q: %w", srcSchema, err)
	}
	log.Info("Found legacy tables", slog.Int("count", len(tables)))

	// ── 2. Seed device_types dictionary (required by FK before inserting devices)
	if err := s.seedDeviceTypes(ctx); err != nil {
		return fmt.Errorf("seed device_types: %w", err)
	}

	// ── 3. Parse table names, skip unparseable ones ───────────────────────────
	var parsed []*ParsedTable
	for _, t := range tables {
		p, err := ParseTableName(t)
		if err != nil {
			log.Warn("Skipping unparseable table", slog.String("table", t), slog.Any("error", err))
			continue
		}
		parsed = append(parsed, p)
	}
	log.Info("Parsed device tables", slog.Int("count", len(parsed)))

	// ── 3. Collect unique sites and gateways ──────────────────────────────────
	// Use maps to deduplicate before hitting the DB.
	siteMap := make(map[string]*model.Site)        // utility_id → Site
	gatewayMap := make(map[string]*model.Gateway)  // GatewayKey → Gateway
	defaultZoneMap := make(map[string]*model.Zone) // utility_id → default Zone

	for _, p := range parsed {
		if _, ok := siteMap[p.UtilityID]; !ok {
			siteMap[p.UtilityID] = &model.Site{
				UtilityID: p.UtilityID,
				NameCN:    p.UtilityID, // placeholder — update manually after seeding
				SiteCode:  p.UtilityID,
			}
		}
		key := p.GatewayKey()
		if _, ok := gatewayMap[key]; !ok {
			gatewayMap[key] = &model.Gateway{
				SerialNo:      key,
				Mac:           key, // placeholder — update manually after seeding
				Model:         "legacy",
				DisplayName:   key,
				Status:        "offline",
				NetworkStatus: "offline",
				MQTTUsername:  key,
			}
		}
	}

	// ── 4. Upsert sites ───────────────────────────────────────────────────────
	for _, site := range siteMap {
		if err := s.upsertSite(ctx, site); err != nil {
			return fmt.Errorf("upsert site %q: %w", site.UtilityID, err)
		}
		log.Info("Site ready", slog.String("utility_id", site.UtilityID), slog.String("id", site.ID.String()))
	}

	// ── 5. Upsert default zones (one per site, used as placeholder for devices) ─
	for _, site := range siteMap {
		zone := &model.Zone{
			SiteID:       site.ID,
			ZoneName:     "default",
			DisplayOrder: 0,
		}
		if err := s.dstDB.WithContext(ctx).
			Where(model.Zone{SiteID: site.ID, ZoneName: "default"}).
			FirstOrCreate(zone).Error; err != nil {
			return fmt.Errorf("upsert default zone for site %q: %w", site.UtilityID, err)
		}
		defaultZoneMap[site.UtilityID] = zone
		log.Info("Default zone ready", slog.String("utility_id", site.UtilityID), slog.String("zone_id", zone.ID.String()))
	}

	// ── 6. Upsert gateways (now that site IDs are known) ─────────────────────
	for _, p := range parsed {
		gw := gatewayMap[p.GatewayKey()]
		if gw.SiteID == uuid.Nil {
			gw.SiteID = siteMap[p.UtilityID].ID
		}
	}
	for _, gw := range gatewayMap {
		if err := s.upsertGateway(ctx, gw); err != nil {
			return fmt.Errorf("upsert gateway %q: %w", gw.SerialNo, err)
		}
		log.Info("Gateway ready", slog.String("serial_no", gw.SerialNo), slog.String("id", gw.ID.String()))
	}

	// ── 7. Upsert devices + build etl_table_map ───────────────────────────────
	for _, p := range parsed {
		// Skip GW tables — they are operational data, not telemetry.
		if strings.ToLower(p.DeviceType) == "gw" {
			log.Info("Skipping GW table", slog.String("table", p.OldTableName))
			continue
		}

		gw := gatewayMap[p.GatewayKey()]
		site := siteMap[p.UtilityID]
		defaultZone := defaultZoneMap[p.UtilityID]

		entry, err := s.upsertDeviceAndMapping(ctx, p, gw, site, defaultZone)
		if err != nil {
			return fmt.Errorf("upsert device for table %q: %w", p.OldTableName, err)
		}

		log.Info("Device mapped",
			slog.String("table", p.OldTableName),
			slog.String("type", p.DeviceType),
			slog.Any("device_id", entry.DeviceID),
			slog.Any("assignment_id", entry.AssignmentID),
		)
	}

	log.Info("MetaSeeder completed successfully")
	return nil
}

// ── private helpers ───────────────────────────────────────────────────────────

func (s *MetaSeeder) listTables(ctx context.Context, schema string) ([]string, error) {
	var rows []TableRow
	err := s.srcDB.WithContext(ctx).Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ?
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`, schema).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.TableName
	}
	return names, nil
}

// upsertSite inserts the site if it doesn't exist, or loads its ID if it does.
// Uses utility_id as the unique key.
func (s *MetaSeeder) upsertSite(ctx context.Context, site *model.Site) error {
	result := s.dstDB.WithContext(ctx).
		Where(model.Site{UtilityID: site.UtilityID}).
		FirstOrCreate(site)
	return result.Error
}

// upsertGateway inserts the gateway if it doesn't exist, or loads its ID.
// Uses serial_no as the unique key.
func (s *MetaSeeder) upsertGateway(ctx context.Context, gw *model.Gateway) error {
	result := s.dstDB.WithContext(ctx).
		Where(model.Gateway{SerialNo: gw.SerialNo}).
		FirstOrCreate(gw)
	return result.Error
}

// upsertDeviceAndMapping creates (or finds) the Device and, for sensor types,
// the PhysicalPoint + PointAssignment. It then writes the etl_table_map entry.
func (s *MetaSeeder) upsertDeviceAndMapping(
	ctx context.Context,
	p *ParsedTable,
	gw *model.Gateway,
	_ *model.Site,
	defaultZone *model.Zone,
) (*ETLTableMap, error) {
	// device_code encodes loop+slave+pin so it is unique within a gateway.
	deviceCode := fmt.Sprintf("%d%s%d", p.Loop, p.SlaveHex, p.Pin)

	device := &model.Device{
		GatewayID:      gw.ID,
		ZoneID:         defaultZone.ID, // placeholder — reassign zones manually after seeding
		DeviceTypeCode: strings.ToUpper(p.DeviceType),
		DeviceCode:     deviceCode,
		FuncTag:        fmt.Sprintf("%s_%s", strings.ToUpper(p.DeviceType), deviceCode),
	}

	if err := s.dstDB.WithContext(ctx).
		Where(model.Device{GatewayID: gw.ID, DeviceCode: deviceCode}).
		FirstOrCreate(device).Error; err != nil {
		return nil, fmt.Errorf("upsert device: %w", err)
	}

	entry := &ETLTableMap{
		OldTableName: p.OldTableName,
		UtilityID:    p.UtilityID,
		DeviceType:   p.DeviceType,
	}

	if p.IsSensor() {
		// Sensor types need PhysicalPoint + PointAssignment for telemetry_sensors FK.
		assignment, err := s.upsertSensorAssignment(ctx, device, p)
		if err != nil {
			return nil, err
		}
		entry.AssignmentID = &assignment.ID
	} else {
		entry.DeviceID = &device.ID
	}

	// Upsert into etl_table_map (idempotent).
	if err := s.dstDB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "old_table_name"}},
			DoUpdates: clause.AssignmentColumns([]string{"device_id", "assignment_id"}),
		}).
		Create(entry).Error; err != nil {
		return nil, fmt.Errorf("upsert etl_table_map: %w", err)
	}

	return entry, nil
}

// upsertSensorAssignment creates the PhysicalPoint and PointAssignment records
// needed for sensor types (ST, SP, SR, SO).
func (s *MetaSeeder) upsertSensorAssignment(
	ctx context.Context,
	device *model.Device,
	p *ParsedTable,
) (*model.PointAssignment, error) {
	point := &model.PhysicalPoint{
		DeviceID:  device.ID,
		PortIndex: p.Pin,
	}
	if err := s.dstDB.WithContext(ctx).
		Where(model.PhysicalPoint{DeviceID: device.ID, PortIndex: p.Pin}).
		FirstOrCreate(point).Error; err != nil {
		return nil, fmt.Errorf("upsert physical_point: %w", err)
	}

	now := time.Now()
	assignment := &model.PointAssignment{
		PointID:        point.ID,
		ZoneID:         device.ZoneID,
		SensorTypeCode: strings.ToUpper(p.DeviceType),
		SensorName:     fmt.Sprintf("%s_%d%s%d", strings.ToUpper(p.DeviceType), p.Loop, p.SlaveHex, p.Pin),
		FuncTag:        fmt.Sprintf("%s_%d%s%d", strings.ToUpper(p.DeviceType), p.Loop, p.SlaveHex, p.Pin),
		ActiveFrom:     &now,
	}
	if err := s.dstDB.WithContext(ctx).
		Where(model.PointAssignment{PointID: point.ID, SensorTypeCode: strings.ToUpper(p.DeviceType)}).
		FirstOrCreate(assignment).Error; err != nil {
		return nil, fmt.Errorf("upsert point_assignment: %w", err)
	}

	return assignment, nil
}

// seedDeviceTypes ensures the device_types dictionary contains all codes
// referenced by the legacy ima_thing table names.
// This is idempotent — existing rows are skipped.
func (s *MetaSeeder) seedDeviceTypes(ctx context.Context) error {
	types := []model.DeviceType{
		{Code: "CI", Description: "Inverter", Category: "Device"},
		{Code: "SE", Description: "Power Meter", Category: "Device"},
		{Code: "SF", Description: "Flow Meter", Category: "Device"},
		{Code: "ST", Description: "Temperature Sensor", Category: "Sensor"},
		{Code: "SP", Description: "Pressure Sensor", Category: "Sensor"},
		{Code: "SR", Description: "Digital Sensor", Category: "Sensor"},
		{Code: "SO", Description: "Oxygen Sensor", Category: "Sensor"},
		{Code: "SH", Description: "PH Sensor", Category: "Sensor"},
		{Code: "SW", Description: "Weight Sensor", Category: "Sensor"},
		{Code: "GW", Description: "Gateway (operational, not telemetry)", Category: "Gateway"},
	}
	for _, dt := range types {
		if err := s.dstDB.WithContext(ctx).
			Where(model.DeviceType{Code: dt.Code}).
			FirstOrCreate(&dt).Error; err != nil {
			return fmt.Errorf("upsert device_type %q: %w", dt.Code, err)
		}
	}
	slog.Default().With(slog.String("component", "MetaSeeder")).
		Info("device_types seeded", slog.Int("count", len(types)))
	return nil
}
