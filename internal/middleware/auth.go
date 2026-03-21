package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const identityKey = "identity"

// Identity holds the authenticated caller's details.
type Identity struct {
	UserID      string
	Roles       []string
	Permissions []string
}

// HasPermission reports whether the identity holds the given permission.
func (id *Identity) HasPermission(perm string) bool {
	for _, p := range id.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// Authenticator validates a bearer token and returns the caller's Identity.
// Implement this interface to swap in a real auth backend (e.g. JWT, OAuth2
// introspection) without changing any routing code.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*Identity, error)
}

// Auth returns a Gin middleware that extracts the Bearer token from the
// Authorization header, authenticates it via authn, and stores the resulting
// Identity in the context under the "identity" key.
//
// On failure it aborts with 401 { "error": "unauthorized" }.
func Auth(authn Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" || token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		identity, err := authn.Authenticate(c.Request.Context(), token)
		if err != nil || identity == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(identityKey, identity)
		c.Next()
	}
}

// RequirePermission returns a Gin middleware that checks whether the Identity
// stored in the context holds the given permission.
//
// It must be placed after Auth() in the middleware chain.
// On failure it aborts with 403 { "error": "forbidden" }.
func RequirePermission(perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		identity, ok := GetIdentity(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		if !identity.HasPermission(perm) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}

// GetIdentity safely retrieves the Identity stored by Auth() from the context.
func GetIdentity(c *gin.Context) (*Identity, bool) {
	v, exists := c.Get(identityKey)
	if !exists {
		return nil, false
	}
	identity, ok := v.(*Identity)
	return identity, ok
}
