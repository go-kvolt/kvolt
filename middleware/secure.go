package middleware

import (
	"fmt"

	"github.com/go-kvolt/kvolt/context"
)

// SecureConfig defines the config for Secure middleware.
type SecureConfig struct {
	// XSSProtection sets the X-XSS-Protection header val. Default: "1; mode=block".
	XSSProtection string
	// ContentTypeNosniff sets the X-Content-Type-Options header val. Default: "nosniff".
	ContentTypeNosniff string
	// XFrameOptions sets the X-Frame-Options header val. Default: "DENY".
	XFrameOptions string
	// HSTSMaxAge sets the Strict-Transport-Security header max age in seconds. Default: 0 (disabled).
	HSTSMaxAge int
	// HSTSExcludeSubdomains sets the Strict-Transport-Security header exclude subdomains. Default: false.
	HSTSExcludeSubdomains bool
	// ContentSecurityPolicy sets the Content-Security-Policy header val. Default: "".
	ContentSecurityPolicy string
}

// DefaultSecureConfig is the default Secure middleware config.
var DefaultSecureConfig = SecureConfig{
	XSSProtection:      "1; mode=block",
	ContentTypeNosniff: "nosniff",
	XFrameOptions:      "DENY",
	HSTSMaxAge:         0,
}

// Secure returns a middleware that sets security headers.
func Secure() func(c *context.Context) error {
	return SecureWithConfig(DefaultSecureConfig)
}

// SecureWithConfig returns a middleware that sets security headers with config.
func SecureWithConfig(config SecureConfig) func(c *context.Context) error {
	return func(c *context.Context) error {
		// X-XSS-Protection
		if config.XSSProtection != "" {
			c.Writer.Header().Set("X-XSS-Protection", config.XSSProtection)
		}

		// X-Content-Type-Options
		if config.ContentTypeNosniff != "" {
			c.Writer.Header().Set("X-Content-Type-Options", config.ContentTypeNosniff)
		}

		// X-Frame-Options
		if config.XFrameOptions != "" {
			c.Writer.Header().Set("X-Frame-Options", config.XFrameOptions)
		}

		// HSTS
		if c.Request.TLS != nil && config.HSTSMaxAge != 0 {
			subdomains := ""
			if !config.HSTSExcludeSubdomains {
				subdomains = "; includeSubDomains"
			}
			c.Writer.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d%s", config.HSTSMaxAge, subdomains))
		}

		// CSP
		if config.ContentSecurityPolicy != "" {
			c.Writer.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
		}

		c.Next()
		return nil
	}
}
