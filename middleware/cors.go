package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
)

// CORSConfig configures Cross-Origin Resource Sharing.
type CORSConfig struct {
	// AllowOrigins is a list of origins, or ["*"] for any (credentials disabled with *).
	AllowOrigins []string
	// AllowMethods default includes PATCH and OPTIONS.
	AllowMethods []string
	// AllowHeaders request headers allowed.
	AllowHeaders []string
	// ExposeHeaders response headers browsers may read.
	ExposeHeaders []string
	// AllowCredentials sets Access-Control-Allow-Credentials.
	// Must not be combined with AllowOrigins ["*"].
	AllowCredentials bool
	// MaxAge preflight cache seconds.
	MaxAge int
}

// CORS returns permissive CORS (Allow-Origin *) without credentials.
func CORS() func(c *context.Context) error {
	return CORSWithConfig(CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token", "X-Request-ID", "X-API-Key"},
	})
}

// CORSWithConfig returns CORS middleware with the given config.
func CORSWithConfig(cfg CORSConfig) func(c *context.Context) error {
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}
	methods := strings.Join(cfg.AllowMethods, ", ")
	headers := strings.Join(cfg.AllowHeaders, ", ")
	expose := strings.Join(cfg.ExposeHeaders, ", ")
	wildcard := false
	for _, o := range cfg.AllowOrigins {
		if strings.TrimSpace(o) == "*" {
			wildcard = true
			break
		}
	}
	if wildcard && cfg.AllowCredentials {
		// Browsers reject * + credentials; force credentials off.
		cfg.AllowCredentials = false
	}

	originAllowed := func(origin string) (string, bool) {
		if origin == "" {
			return "", false
		}
		if wildcard {
			return "*", true
		}
		for _, o := range cfg.AllowOrigins {
			if strings.EqualFold(o, origin) {
				return origin, true
			}
		}
		return "", false
	}

	return func(c *context.Context) error {
		if c == nil || c.Request == nil || c.Writer == nil {
			if c != nil {
				c.Next()
			}
			return nil
		}
		origin := c.Request.Header.Get("Origin")
		if allow, ok := originAllowed(origin); ok {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allow)
			if cfg.AllowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if !wildcard {
				c.Writer.Header().Add("Vary", "Origin")
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", methods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
		if expose != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", expose)
		}
		if cfg.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		}

		// Only short-circuit CORS preflight; do not shadow app OPTIONS handlers.
		if c.Request.Method == http.MethodOptions && c.Request.Header.Get("Access-Control-Request-Method") != "" {
			c.Status(http.StatusNoContent)
			c.FlushHeaders()
			return nil
		}
		c.Next()
		return nil
	}
}
