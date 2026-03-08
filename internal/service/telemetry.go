package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/hill/orion/internal/dto"
	"github.com/hill/orion/internal/model"
	"github.com/hill/orion/internal/repository"
)

type TelemetryService interface {
	LatestByDevice(ctx context.Context, deviceID uuid.UUID) (any, error)
	LatestByAssignment(ctx context.Context, assignmentID uuid.UUID) (*dto.LatestSensorResponse, error)
	LatestBySite(ctx context.Context, siteID uuid.UUID) (*dto.SiteLatestResponse, error)
	HistoryByDevice(ctx context.Context, deviceID uuid.UUID, from, to time.Time) (any, error)
	HistoryByAssignment(ctx context.Context, assignmentID uuid.UUID, from, to time.Time) ([]dto.LatestSensorResponse, error)
}

type telemetryService struct {
	repo   repository.TelemetryRepository
	gormDB *gorm.DB
}

func NewTelemetryService(repo repository.TelemetryRepository, gormDB *gorm.DB) TelemetryService {
	return &telemetryService{repo: repo, gormDB: gormDB}
}

// ── LatestByDevice ────────────────────────────────────────────────────────────

func (s *telemetryService) LatestByDevice(ctx context.Context, deviceID uuid.UUID) (any, error) {
	dev, err := s.fetchDeviceWithMeta(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	info := dto.DeviceInfo{
		DeviceID:       dev.ID,
		DeviceTypeCode: dev.DeviceTypeCode,
		FuncTag:        dev.FuncTag,
		DeviceCode:     dev.DeviceCode,
		Zone: dto.ZoneBrief{
			ID:       dev.Zone.ID,
			ZoneName: dev.Zone.ZoneName,
		},
		Site: dto.SiteBrief{
			ID:        dev.Zone.Site.ID,
			UtilityID: dev.Zone.Site.UtilityID,
			NameCN:    dev.Zone.Site.NameCN,
		},
	}

	switch dev.DeviceTypeCode {
	case "SE":
		row, err := s.repo.LatestMeter(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		return &dto.LatestMeterResponse{
			DeviceInfo: info,
			TS:         row.TS,
			Voltage:    row.Voltage, Current: row.Current,
			KW: row.KW, KVA: row.KVA, KVAR: row.KVAR,
			KWH: row.KWH, KVAH: row.KVAH, KVARH: row.KVARH,
			CurrentA: row.CurrentA, CurrentB: row.CurrentB, CurrentC: row.CurrentC,
			PF: row.PF, Status: row.Status,
		}, nil

	case "CI":
		row, err := s.repo.LatestInverter(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		return &dto.LatestInverterResponse{
			DeviceInfo: info,
			TS:         row.TS,
			Voltage:    row.Voltage, Current: row.Current,
			KW: row.KW, KWH: row.KWH, HZ: row.HZ,
			Error: row.Error, Alert: row.Alert,
			InvStatus: row.InvStatus, Status: row.Status,
		}, nil

	case "SF":
		row, err := s.repo.LatestFlowMeter(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		return &dto.LatestFlowMeterResponse{
			DeviceInfo:     info,
			TS:             row.TS,
			Flow:           row.Flow,
			Consumption:    row.Consumption,
			RevConsumption: row.RevConsumption,
			Direction:      row.Direction,
			Status:         row.Status,
		}, nil

	default:
		return nil, fmt.Errorf("device type %q does not have a telemetry table", dev.DeviceTypeCode)
	}
}

// ── LatestByAssignment ────────────────────────────────────────────────────────

func (s *telemetryService) LatestByAssignment(ctx context.Context, assignmentID uuid.UUID) (*dto.LatestSensorResponse, error) {
	row, err := s.repo.LatestSensor(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	return &dto.LatestSensorResponse{
		TS:           row.TS,
		AssignmentID: row.AssignmentID,
		Val:          row.Val,
		Status:       row.Status,
	}, nil
}

// ── LatestBySite ──────────────────────────────────────────────────────────────

func (s *telemetryService) LatestBySite(ctx context.Context, siteID uuid.UUID) (*dto.SiteLatestResponse, error) {
	// 1. Load metadata tree: site → zones → devices + point_assignments
	var site model.Site
	err := s.gormDB.WithContext(ctx).
		Preload("Zones.Devices").
		Preload("Zones.Devices.PhysicalPoints.PointAssignments").
		First(&site, "id = ?", siteID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("site %s not found", siteID)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch site metadata: %w", err)
	}

	// 2. Collect device IDs by type and all assignment IDs
	seIDs, ciIDs, sfIDs := []uuid.UUID{}, []uuid.UUID{}, []uuid.UUID{}
	assignmentIDs := []uuid.UUID{}

	for _, zone := range site.Zones {
		for _, dev := range zone.Devices {
			switch dev.DeviceTypeCode {
			case "SE":
				seIDs = append(seIDs, dev.ID)
			case "CI":
				ciIDs = append(ciIDs, dev.ID)
			case "SF":
				sfIDs = append(sfIDs, dev.ID)
			}
			for _, pp := range dev.PhysicalPoints {
				for _, pa := range pp.PointAssignments {
					assignmentIDs = append(assignmentIDs, pa.ID)
				}
			}
		}
	}

	// 3. Bulk fetch latest telemetry (4 queries total regardless of device count)
	meters, err := s.repo.LatestMetersByIDs(ctx, seIDs)
	if err != nil {
		return nil, err
	}
	inverters, err := s.repo.LatestInvertersByIDs(ctx, ciIDs)
	if err != nil {
		return nil, err
	}
	flowMeters, err := s.repo.LatestFlowMetersByIDs(ctx, sfIDs)
	if err != nil {
		return nil, err
	}
	sensors, err := s.repo.LatestSensorsByIDs(ctx, assignmentIDs)
	if err != nil {
		return nil, err
	}

	// 4. Build response tree
	siteBrief := dto.SiteBrief{
		ID:        site.ID,
		UtilityID: site.UtilityID,
		NameCN:    site.NameCN,
	}

	zones := make([]dto.ZoneLatestGroup, 0, len(site.Zones))
	for _, zone := range site.Zones {
		group := dto.ZoneLatestGroup{
			ZoneID:      zone.ID,
			ZoneName:    zone.ZoneName,
			Devices:     make([]dto.DeviceLatestEntry, 0, len(zone.Devices)),
			Assignments: []dto.AssignmentLatestEntry{},
		}

		for _, dev := range zone.Devices {
			entry := dto.DeviceLatestEntry{
				DeviceID:       dev.ID,
				DeviceTypeCode: dev.DeviceTypeCode,
				FuncTag:        dev.FuncTag,
				DeviceCode:     dev.DeviceCode,
			}

			switch dev.DeviceTypeCode {
			case "SE":
				if row, ok := meters[dev.ID]; ok {
					ts := row.TS
					entry.TS = &ts
					entry.Data = dto.MeterData{
						Voltage: row.Voltage, Current: row.Current,
						KW: row.KW, KVA: row.KVA, KVAR: row.KVAR,
						KWH: row.KWH, KVAH: row.KVAH, KVARH: row.KVARH,
						CurrentA: row.CurrentA, CurrentB: row.CurrentB, CurrentC: row.CurrentC,
						PF: row.PF, Status: row.Status,
					}
				}
			case "CI":
				if row, ok := inverters[dev.ID]; ok {
					ts := row.TS
					entry.TS = &ts
					entry.Data = dto.InverterData{
						Voltage: row.Voltage, Current: row.Current,
						KW: row.KW, KWH: row.KWH, HZ: row.HZ,
						Error: row.Error, Alert: row.Alert,
						InvStatus: row.InvStatus, Status: row.Status,
					}
				}
			case "SF":
				if row, ok := flowMeters[dev.ID]; ok {
					ts := row.TS
					entry.TS = &ts
					entry.Data = dto.FlowMeterData{
						Flow:           row.Flow,
						Consumption:    row.Consumption,
						RevConsumption: row.RevConsumption,
						Direction:      row.Direction,
						Status:         row.Status,
					}
				}
			}
			group.Devices = append(group.Devices, entry)

			// Assignments under this device's physical points
			for _, pp := range dev.PhysicalPoints {
				for _, pa := range pp.PointAssignments {
					aEntry := dto.AssignmentLatestEntry{
						AssignmentID:   pa.ID,
						SensorTypeCode: pa.SensorTypeCode,
						FuncTag:        pa.FuncTag,
						SensorName:     pa.SensorName,
						Unit:           pa.Unit,
					}
					if row, ok := sensors[pa.ID]; ok {
						ts := row.TS
						aEntry.TS = &ts
						aEntry.Val = row.Val
						aEntry.Status = row.Status
					}
					group.Assignments = append(group.Assignments, aEntry)
				}
			}
		}
		zones = append(zones, group)
	}

	_ = siteBrief // used in DeviceInfo for single-device but not needed here at top level
	return &dto.SiteLatestResponse{
		SiteID:    site.ID,
		UtilityID: site.UtilityID,
		NameCN:    site.NameCN,
		Zones:     zones,
	}, nil
}

// ── History ───────────────────────────────────────────────────────────────────

func (s *telemetryService) HistoryByDevice(ctx context.Context, deviceID uuid.UUID, from, to time.Time) (any, error) {
	dev, err := s.fetchDeviceWithMeta(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	switch dev.DeviceTypeCode {
	case "SE":
		rows, err := s.repo.MeterHistory(ctx, deviceID, from, to)
		if err != nil {
			return nil, err
		}
		out := make([]dto.LatestMeterResponse, len(rows))
		for i, row := range rows {
			out[i] = dto.LatestMeterResponse{TS: row.TS,
				Voltage: row.Voltage, Current: row.Current,
				KW: row.KW, KVA: row.KVA, KVAR: row.KVAR,
				KWH: row.KWH, KVAH: row.KVAH, KVARH: row.KVARH,
				CurrentA: row.CurrentA, CurrentB: row.CurrentB, CurrentC: row.CurrentC,
				PF: row.PF, Status: row.Status,
			}
		}
		return out, nil

	case "CI":
		rows, err := s.repo.InverterHistory(ctx, deviceID, from, to)
		if err != nil {
			return nil, err
		}
		out := make([]dto.LatestInverterResponse, len(rows))
		for i, row := range rows {
			out[i] = dto.LatestInverterResponse{TS: row.TS,
				Voltage: row.Voltage, Current: row.Current,
				KW: row.KW, KWH: row.KWH, HZ: row.HZ,
				Error: row.Error, Alert: row.Alert,
				InvStatus: row.InvStatus, Status: row.Status,
			}
		}
		return out, nil

	case "SF":
		rows, err := s.repo.FlowMeterHistory(ctx, deviceID, from, to)
		if err != nil {
			return nil, err
		}
		out := make([]dto.LatestFlowMeterResponse, len(rows))
		for i, row := range rows {
			out[i] = dto.LatestFlowMeterResponse{TS: row.TS,
				Flow:           row.Flow,
				Consumption:    row.Consumption,
				RevConsumption: row.RevConsumption,
				Direction:      row.Direction,
				Status:         row.Status,
			}
		}
		return out, nil

	default:
		return nil, fmt.Errorf("device type %q does not have a telemetry table", dev.DeviceTypeCode)
	}
}

func (s *telemetryService) HistoryByAssignment(ctx context.Context, assignmentID uuid.UUID, from, to time.Time) ([]dto.LatestSensorResponse, error) {
	rows, err := s.repo.SensorHistory(ctx, assignmentID, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]dto.LatestSensorResponse, len(rows))
	for i, row := range rows {
		out[i] = dto.LatestSensorResponse{
			TS:           row.TS,
			AssignmentID: row.AssignmentID,
			Val:          row.Val,
			Status:       row.Status,
		}
	}
	return out, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// fetchDeviceWithMeta loads device + zone + site in one GORM query.
func (s *telemetryService) fetchDeviceWithMeta(ctx context.Context, deviceID uuid.UUID) (*model.Device, error) {
	var dev model.Device
	err := s.gormDB.WithContext(ctx).
		Preload("Zone.Site").
		First(&dev, "id = ?", deviceID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("device %s not found", deviceID)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch device: %w", err)
	}
	return &dev, nil
}
