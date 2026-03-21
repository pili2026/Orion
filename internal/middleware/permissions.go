// Package middleware provides HTTP middleware for the Orion platform.
package middleware

// Permission constants used across the application.
const (
	PermSiteRead      = "site:read"
	PermSiteWrite     = "site:write"
	PermSiteDelete    = "site:delete"
	PermGatewayRead   = "gateway:read"
	PermGatewayWrite  = "gateway:write"
	PermGatewayDelete = "gateway:delete"
	PermTelemetryRead = "telemetry:read"
	PermDashboardManage = "dashboard:manage"
	PermTopologyRead   = "topology:read"
	PermUserManage     = "user:manage"
)
