package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// CSRFConfig configures double-submit CSRF protection.
type CSRFConfig struct {
	CookieName string
	HeaderName string
	CookiePath string
	// SameSite cookie attribute. Default: Lax.
	SameSite http.SameSite
	// Secure forces the Secure cookie flag. Ignored when ManualSecure is false (default):
	// then Secure is set automatically for HTTPS requests (TLS or trusted X-Forwarded-Proto).
	Secure bool
	// ManualSecure when true uses the Secure field as-is (no auto HTTPS detection).
	ManualSecure bool
	// InsecureCookie forces Secure=false (local HTTP only). Overrides auto/ManualSecure.
	InsecureCookie bool
	Skipper        func(c *context.Context) bool
	SkipPaths      []string
}

// CSRF returns middleware using default CSRFConfig.
func CSRF() func(c *context.Context) error {
	return CSRFWithConfig(CSRFConfig{})
}

// CSRFWithConfig returns double-submit cookie CSRF middleware.
func CSRFWithConfig(cfg CSRFConfig) func(c *context.Context) error {
	if cfg.CookieName == "" {
		cfg.CookieName = "csrf_token"
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-CSRF-Token"
	}
	if cfg.CookiePath == "" {
		cfg.CookiePath = "/"
	}
	if cfg.SameSite == 0 {
		cfg.SameSite = http.SameSiteLaxMode
	}

	safe := map[string]bool{
		http.MethodGet: true, http.MethodHead: true,
		http.MethodOptions: true, http.MethodTrace: true,
	}

	return func(c *context.Context) error {
		if cfg.Skipper != nil && cfg.Skipper(c) {
			c.Next()
			return nil
		}
		path := ""
		if c.Request != nil && c.Request.URL != nil {
			path = c.Request.URL.Path
		}
		for _, p := range cfg.SkipPaths {
			p = strings.TrimSpace(p)
			if p == "" {
				continue // empty entry would match every path via HasPrefix(path, "/")
			}
			if path == p || strings.HasPrefix(path, p+"/") {
				c.Next()
				return nil
			}
		}

		token := ""
		if cookie, err := c.Request.Cookie(cfg.CookieName); err == nil {
			token = cookie.Value
		}

		if safe[c.Request.Method] {
			if token == "" {
				token = newCSRFToken()
				c.SetCookieWith(cfg.CookieName, token, context.CookieOptions{
					Path:     cfg.CookiePath,
					MaxAge:   86400,
					Secure:   csrfSecureFlag(c, cfg),
					HTTPOnly: false, // must be readable by JS for double-submit
					SameSite: cfg.SameSite,
				})
			}
			c.Next()
			return nil
		}

		got := c.Request.Header.Get(cfg.HeaderName)
		if got == "" {
			got = c.Request.FormValue("_csrf")
		}
		if got == "" {
			got = c.Request.FormValue("csrf_token")
		}
		if !csrfTokenMatch(token, got) {
			return httperr.Forbidden("csrf_mismatch", "csrf token mismatch")
		}
		c.Next()
		return nil
	}
}

func csrfSecureFlag(c *context.Context, cfg CSRFConfig) bool {
	if cfg.InsecureCookie {
		return false
	}
	if cfg.ManualSecure {
		return cfg.Secure
	}
	return isRequestHTTPS(c)
}

func isRequestHTTPS(c *context.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.TLS != nil {
		return true
	}
	if strings.EqualFold(c.Request.Header.Get("X-Forwarded-Proto"), "https") && peerTrusted(c) {
		return true
	}
	return false
}

func csrfTokenMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	// Pad to equal length for constant-time compare without leaking length via early return on Compare.
	if len(a) != len(b) {
		// Still compare against itself to keep roughly constant work; result is failure.
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func newCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Extremely rare; fall back to zero buffer is unsafe — panic so misconfig is loud.
		panic("csrf: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
