package middleware

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
)

// SecureConfig defines the config for Secure middleware.
type SecureConfig struct {
	XSSProtection         string
	ContentTypeNosniff    string
	XFrameOptions         string
	HSTSMaxAge            int
	HSTSExcludeSubdomains bool
	HSTSPreload           bool
	// HSTSIfXForwardedProto sets HSTS when X-Forwarded-Proto is https.
	// Only honored when the peer is in Context.TrustedProxies (set via Engine.TrustedProxies).
	HSTSIfXForwardedProto bool
	ContentSecurityPolicy string
	ReferrerPolicy        string
	PermissionsPolicy     string
}

// DefaultSecureConfig is the default Secure middleware config.
var DefaultSecureConfig = SecureConfig{
	XSSProtection:         "0",
	ContentTypeNosniff:    "nosniff",
	XFrameOptions:         "DENY",
	HSTSMaxAge:            0,
	HSTSIfXForwardedProto: true,
	ReferrerPolicy:        "strict-origin-when-cross-origin",
}

// Secure returns a middleware that sets security headers.
func Secure() func(c *context.Context) error {
	return SecureWithConfig(DefaultSecureConfig)
}

// SecureWithConfig returns a middleware that sets security headers with config.
func SecureWithConfig(config SecureConfig) func(c *context.Context) error {
	return func(c *context.Context) error {
		if config.XSSProtection != "" {
			c.Writer.Header().Set("X-XSS-Protection", config.XSSProtection)
		}
		if config.ContentTypeNosniff != "" {
			c.Writer.Header().Set("X-Content-Type-Options", config.ContentTypeNosniff)
		}
		if config.XFrameOptions != "" {
			c.Writer.Header().Set("X-Frame-Options", config.XFrameOptions)
		}
		if config.ReferrerPolicy != "" {
			c.Writer.Header().Set("Referrer-Policy", config.ReferrerPolicy)
		}
		if config.PermissionsPolicy != "" {
			c.Writer.Header().Set("Permissions-Policy", config.PermissionsPolicy)
		}

		https := c.Request.TLS != nil
		if !https && config.HSTSIfXForwardedProto &&
			strings.EqualFold(c.Request.Header.Get("X-Forwarded-Proto"), "https") &&
			peerTrusted(c) {
			https = true
		}
		if https && config.HSTSMaxAge > 0 {
			val := fmt.Sprintf("max-age=%d", config.HSTSMaxAge)
			if !config.HSTSExcludeSubdomains {
				val += "; includeSubDomains"
			}
			if config.HSTSPreload {
				val += "; preload"
			}
			c.Writer.Header().Set("Strict-Transport-Security", val)
		}

		if config.ContentSecurityPolicy != "" {
			c.Writer.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
		}

		c.Next()
		return nil
	}
}

func peerTrusted(c *context.Context) bool {
	if c == nil || c.Request == nil || len(c.TrustedProxies) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, t := range c.TrustedProxies {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.Contains(t, "/") {
			_, cidr, err := net.ParseCIDR(t)
			if err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if tip := net.ParseIP(t); tip != nil && tip.Equal(ip) {
			return true
		}
	}
	return false
}
