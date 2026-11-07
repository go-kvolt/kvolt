package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// CSRFConfig configures HMAC-signed double-submit CSRF protection.
type CSRFConfig struct {
	// Secret signs tokens so a planted cookie without the secret cannot pass.
	// When empty, a process-local random secret is used (fine for single-instance).
	Secret string
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

var (
	csrfProcessSecret     []byte
	csrfProcessSecretOnce sync.Once
)

func csrfSecretBytes(cfg CSRFConfig) []byte {
	if s := strings.TrimSpace(cfg.Secret); s != "" {
		return []byte(s)
	}
	csrfProcessSecretOnce.Do(func() {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("csrf: crypto/rand failed: " + err.Error())
		}
		csrfProcessSecret = b
	})
	return csrfProcessSecret
}

// CSRF returns middleware using default CSRFConfig.
func CSRF() func(c *context.Context) error {
	return CSRFWithConfig(CSRFConfig{})
}

// CSRFWithConfig returns signed double-submit cookie CSRF middleware.
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
	secret := csrfSecretBytes(cfg)

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
		if token != "" && !csrfTokenValid(secret, token) {
			token = ""
		}

		if safe[c.Request.Method] {
			if token == "" {
				token = newCSRFToken(secret)
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
		if !csrfTokenValid(secret, token) || !csrfTokenMatch(token, got) {
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
	if len(a) != len(b) {
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func newCSRFToken(secret []byte) string {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		panic("csrf: crypto/rand failed: " + err.Error())
	}
	n := hex.EncodeToString(nonce)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(n))
	return n + "." + hex.EncodeToString(mac.Sum(nil))
}

func csrfTokenValid(secret []byte, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(parts[1])) == 1
}
