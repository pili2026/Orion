package handler

import (
	"context"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"github.com/hill/orion/internal/dto"
)

// ── MQTT mock ─────────────────────────────────────────────────────────────────

type mockToken struct{ mqtt.Token }

func (m *mockToken) Wait() bool   { return true }
func (m *mockToken) Error() error { return nil }

type mockMQTTClient struct {
	mqtt.Client
	SubscribedTopics []string
	PublishedTopic   string
	PublishedPayload interface{}
}

func (m *mockMQTTClient) Subscribe(topic string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	m.SubscribedTopics = append(m.SubscribedTopics, topic)
	return &mockToken{}
}

func (m *mockMQTTClient) Publish(topic string, _ byte, _ bool, payload interface{}) mqtt.Token {
	m.PublishedTopic = topic
	m.PublishedPayload = payload
	return &mockToken{}
}

func (m *mockMQTTClient) AddRoute(_ string, _ mqtt.MessageHandler) {}

// ── MQTTIngestService mock ────────────────────────────────────────────────────

type mockMQTTIngestService struct {
	ProcessTelemetryFn func(ctx context.Context, mqttUsername string, payload []byte) error
}

func (m *mockMQTTIngestService) ProcessTelemetry(ctx context.Context, mqttUsername string, payload []byte) error {
	if m.ProcessTelemetryFn != nil {
		return m.ProcessTelemetryFn(ctx, mqttUsername, payload)
	}
	return nil
}

// ── GatewayService mock ───────────────────────────────────────────────────────

type mockGatewayService struct {
	RegisterFn func(ctx context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error)
	ListFn     func(ctx context.Context) ([]dto.GatewayResponse, error)
	GetByIDFn  func(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error)
	UpdateFn   func(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error)
	DeleteFn   func(ctx context.Context, id uuid.UUID) error
}

func (m *mockGatewayService) Register(ctx context.Context, req dto.CreateGatewayRequest) (*dto.RegisterGatewayResponse, error) {
	return m.RegisterFn(ctx, req)
}
func (m *mockGatewayService) List(ctx context.Context) ([]dto.GatewayResponse, error) {
	return m.ListFn(ctx)
}
func (m *mockGatewayService) GetByID(ctx context.Context, id uuid.UUID) (*dto.GatewayResponse, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *mockGatewayService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateGatewayRequest) (*dto.GatewayResponse, error) {
	return m.UpdateFn(ctx, id, req)
}
func (m *mockGatewayService) Delete(ctx context.Context, id uuid.UUID) error {
	return m.DeleteFn(ctx, id)
}

// ── TelemetryService mock ─────────────────────────────────────────────────────

type mockTelemetryService struct {
	LatestByDeviceFn      func(ctx context.Context, deviceID uuid.UUID) (any, error)
	LatestByAssignmentFn  func(ctx context.Context, assignmentID uuid.UUID) (*dto.LatestSensorResponse, error)
	LatestBySiteFn        func(ctx context.Context, siteID uuid.UUID) (*dto.SiteLatestResponse, error)
	HistoryByDeviceFn     func(ctx context.Context, deviceID uuid.UUID, from, to time.Time) (any, error)
	HistoryByAssignmentFn func(ctx context.Context, assignmentID uuid.UUID, from, to time.Time) ([]dto.LatestSensorResponse, error)
}

func (m *mockTelemetryService) LatestByDevice(ctx context.Context, id uuid.UUID) (any, error) {
	return m.LatestByDeviceFn(ctx, id)
}
func (m *mockTelemetryService) LatestByAssignment(ctx context.Context, id uuid.UUID) (*dto.LatestSensorResponse, error) {
	return m.LatestByAssignmentFn(ctx, id)
}
func (m *mockTelemetryService) LatestBySite(ctx context.Context, id uuid.UUID) (*dto.SiteLatestResponse, error) {
	return m.LatestBySiteFn(ctx, id)
}
func (m *mockTelemetryService) HistoryByDevice(ctx context.Context, id uuid.UUID, from, to time.Time) (any, error) {
	return m.HistoryByDeviceFn(ctx, id, from, to)
}
func (m *mockTelemetryService) HistoryByAssignment(ctx context.Context, id uuid.UUID, from, to time.Time) ([]dto.LatestSensorResponse, error) {
	return m.HistoryByAssignmentFn(ctx, id, from, to)
}

// ── SiteService mock ──────────────────────────────────────────────────────────

type mockSiteService struct {
	ListFn    func(ctx context.Context) ([]dto.SiteResponse, error)
	GetByIDFn func(ctx context.Context, id uuid.UUID) (*dto.SiteResponse, error)
	CreateFn  func(ctx context.Context, req dto.CreateSiteRequest) (*dto.SiteResponse, error)
	UpdateFn  func(ctx context.Context, id uuid.UUID, req dto.UpdateSiteRequest) (*dto.SiteResponse, error)
	DeleteFn  func(ctx context.Context, id uuid.UUID) error
}

func (m *mockSiteService) List(ctx context.Context) ([]dto.SiteResponse, error) {
	return m.ListFn(ctx)
}
func (m *mockSiteService) GetByID(ctx context.Context, id uuid.UUID) (*dto.SiteResponse, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *mockSiteService) Create(ctx context.Context, req dto.CreateSiteRequest) (*dto.SiteResponse, error) {
	return m.CreateFn(ctx, req)
}
func (m *mockSiteService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateSiteRequest) (*dto.SiteResponse, error) {
	return m.UpdateFn(ctx, id, req)
}
func (m *mockSiteService) Delete(ctx context.Context, id uuid.UUID) error {
	return m.DeleteFn(ctx, id)
}

// ── ZoneService mock ──────────────────────────────────────────────────────────

type mockZoneService struct {
	ListFn   func(ctx context.Context, siteID uuid.UUID) ([]dto.ZoneResponse, error)
	CreateFn func(ctx context.Context, siteID uuid.UUID, req dto.CreateZoneRequest) (*dto.ZoneResponse, error)
	UpdateFn func(ctx context.Context, siteID, zoneID uuid.UUID, req dto.UpdateZoneRequest) (*dto.ZoneResponse, error)
	DeleteFn func(ctx context.Context, siteID, zoneID uuid.UUID) error
}

func (m *mockZoneService) List(ctx context.Context, siteID uuid.UUID) ([]dto.ZoneResponse, error) {
	return m.ListFn(ctx, siteID)
}
func (m *mockZoneService) Create(ctx context.Context, siteID uuid.UUID, req dto.CreateZoneRequest) (*dto.ZoneResponse, error) {
	return m.CreateFn(ctx, siteID, req)
}
func (m *mockZoneService) Update(ctx context.Context, siteID, zoneID uuid.UUID, req dto.UpdateZoneRequest) (*dto.ZoneResponse, error) {
	return m.UpdateFn(ctx, siteID, zoneID, req)
}
func (m *mockZoneService) Delete(ctx context.Context, siteID, zoneID uuid.UUID) error {
	return m.DeleteFn(ctx, siteID, zoneID)
}

// ── DeviceService mock ────────────────────────────────────────────────────────

type mockDeviceService struct {
	UpdateFn func(ctx context.Context, id uuid.UUID, req dto.UpdateDeviceRequest) (*dto.DeviceResponse, error)
}

func (m *mockDeviceService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateDeviceRequest) (*dto.DeviceResponse, error) {
	return m.UpdateFn(ctx, id, req)
}

// ── Test helper ───────────────────────────────────────────────────────────────

// newTestHandler wires all mocks and returns each for per-test overriding.
// nil DBManager is acceptable for handler-level tests that don't hit the DB.
// The MQTTIngestService mock is wired internally — existing tests do not need
// to control ingest behaviour so it is not exposed as a return value.
func newTestHandler() (*Handler, *mockMQTTClient, *mockGatewayService, *mockTelemetryService, *mockSiteService, *mockZoneService) {
	mqttMock := &mockMQTTClient{}
	gatewaySvc := &mockGatewayService{}
	telemetrySvc := &mockTelemetryService{}
	siteSvc := &mockSiteService{}
	zoneSvc := &mockZoneService{}
	ingestSvc := &mockMQTTIngestService{}
	deviceSvc := &mockDeviceService{}

	h := NewHandler(nil, mqttMock, gatewaySvc, telemetrySvc, siteSvc, zoneSvc, ingestSvc, deviceSvc)
	return h, mqttMock, gatewaySvc, telemetrySvc, siteSvc, zoneSvc
}
