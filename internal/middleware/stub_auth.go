package middleware

import (
	"context"
	"fmt"
)

// StubAuthenticator is a development-only Authenticator that maps well-known
// token strings directly to roles without calling any external service.
// Replace with ExternalAuthenticator in production.
type StubAuthenticator struct {
	tokenToRole map[string]string
	roleToPerms map[string][]string
}

// NewStubAuthenticator returns a StubAuthenticator pre-loaded with the
// standard development tokens.
func NewStubAuthenticator() *StubAuthenticator {
	allPerms := []string{
		PermSiteRead, PermSiteWrite, PermSiteDelete,
		PermGatewayRead, PermGatewayWrite, PermGatewayDelete,
		PermTelemetryRead, PermDashboardManage, PermTopologyRead, PermUserManage,
	}

	return &StubAuthenticator{
		tokenToRole: map[string]string{
			"dev-admin":    "admin",
			"dev-owner":    "owner",
			"dev-workcrew": "work_crew",
			"dev-visitor":  "visitor",
		},
		roleToPerms: map[string][]string{
			"admin": allPerms,
			"owner": {
				PermSiteRead, PermSiteWrite,
				PermGatewayRead, PermGatewayWrite,
				PermTelemetryRead, PermDashboardManage, PermTopologyRead,
			},
			"work_crew": {
				PermSiteRead, PermGatewayRead, PermTelemetryRead, PermTopologyRead,
			},
			"visitor": {
				PermSiteRead,
			},
		},
	}
}

// Authenticate resolves token to an Identity. Returns an error for unknown tokens.
func (s *StubAuthenticator) Authenticate(_ context.Context, token string) (*Identity, error) {
	role, ok := s.tokenToRole[token]
	if !ok {
		return nil, fmt.Errorf("stub: unknown token %q", token)
	}

	perms := s.roleToPerms[role]
	permsCopy := make([]string, len(perms))
	copy(permsCopy, perms)

	return &Identity{
		UserID:      "stub-" + role,
		Roles:       []string{role},
		Permissions: permsCopy,
	}, nil
}
