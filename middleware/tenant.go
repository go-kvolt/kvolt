package middleware

import (
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// TenantIDKey is the context Keys entry for the resolved tenant id.
	TenantIDKey = "tenant_id"
	// DefaultTenantHeader is used when TenantConfig.Header is empty.
	DefaultTenantHeader = "X-Tenant-ID"
)

// TenantConfig configures tenant resolution for multi-tenant apps.
type TenantConfig struct {
	// Header to read (default X-Tenant-ID). Ignored when Extractor is set.
	Header string
	// Param is an optional route param name (e.g. "tenant").
	Param string
	// Claim is an optional JWT claim name read from ContextKey (default "user") MapClaims.
	Claim string
	// ContextKey is the JWT claims context key (default "user").
	ContextKey string
	// Required when true rejects requests with no tenant (400).
	Required bool
	// Extractor overrides Header/Param/Claim resolution when set.
	Extractor func(c *context.Context) (string, error)
	// ErrorHandler overrides the missing-tenant response.
	ErrorHandler func(c *context.Context) error
}

// Tenant resolves a tenant id into c.Keys[TenantIDKey].
// Resolution order when Extractor is nil: Param → Header → Claim.
func Tenant(cfg TenantConfig) func(c *context.Context) error {
	if cfg.Header == "" {
		cfg.Header = DefaultTenantHeader
	}
	if cfg.ContextKey == "" {
		cfg.ContextKey = "user"
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(c *context.Context) error {
			return httperr.BadRequest("tenant_required", "tenant id is required")
		}
	}

	return func(c *context.Context) error {
		id := ""
		var err error
		if cfg.Extractor != nil {
			id, err = cfg.Extractor(c)
			if err != nil {
				return httperr.BadRequest("tenant_invalid", err.Error())
			}
		} else {
			if cfg.Param != "" {
				id = strings.TrimSpace(c.Param(cfg.Param))
			}
			if id == "" && cfg.Header != "" {
				id = strings.TrimSpace(c.Request.Header.Get(cfg.Header))
			}
			if id == "" && cfg.Claim != "" {
				id = tenantFromClaim(c, cfg.ContextKey, cfg.Claim)
			}
		}
		id = strings.TrimSpace(id)
		if id == "" && cfg.Required {
			return writeMiddlewareError(c, cfg.ErrorHandler(c))
		}
		if id != "" {
			c.Set(TenantIDKey, id)
		}
		c.Next()
		return nil
	}
}

// TenantID returns the tenant id stored by Tenant middleware (empty if unset).
func TenantID(c *context.Context) string {
	if c == nil {
		return ""
	}
	v, ok := c.Get(TenantIDKey)
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func tenantFromClaim(c *context.Context, contextKey, claim string) string {
	raw, ok := c.Get(contextKey)
	if !ok || raw == nil {
		return ""
	}
	switch claims := raw.(type) {
	case jwt.MapClaims:
		if v, ok := claims[claim]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	case map[string]interface{}:
		if v, ok := claims[claim]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
