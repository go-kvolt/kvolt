package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// CSRFConfig configures HMAC-signed double-submit CSRF protection.
type CSRFConfig struct {
	// Secret signs tokens so a planted cookie without the secret cannot pass.
	// When empty, a process-local random secret is used (fine for single-instance).
	// In multi-instance deployments, set Secret explicitly so tokens validate across replicas.
	Secret     string
	CookieName string
	HeaderName string
	CookiePath string
	// TokenTTL is the token expiration duration (default: 24h).
	TokenTTL time.Duration
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
	// csrfRandRead is crypto/rand.Read; tests swap it to hit panic paths.
	csrfRandRead = rand.Read
)

func mustCSRFRand(dst []byte) {
	if _, err := csrfRandRead(dst); err != nil {
		panic("csrf: crypto/rand failed: " + err.Error())
	}
}

func csrfSecretBytes(cfg CSRFConfig) []byte {
	if s := strings.TrimSpace(cfg.Secret); s != "" {
		return []byte(s)
	}
	csrfProcessSecretOnce.Do(func() {
		b := make([]byte, 32)
		mustCSRFRand(b)
		csrfProcessSecret = b
		log.Println("[KVolt CSRF] Process-local secret generated; set CSRFConfig.Secret explicitly for multi-instance deployments")
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
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = 24 * time.Hour
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

		now := time.Now()
		token := ""
		if cookie, err := c.Request.Cookie(cfg.CookieName); err == nil {
			token = cookie.Value
		}
		if token != "" && !csrfTokenValid(secret, token, cfg.TokenTTL, now) {
			token = ""
		}

		if safe[c.Request.Method] {
			if token == "" {
				token = newCSRFTokenAt(secret, now)
				c.SetCookieWith(cfg.CookieName, token, context.CookieOptions{
					Path:     cfg.CookiePath,
					MaxAge:   int(cfg.TokenTTL.Seconds()),
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
		if !csrfTokenValid(secret, token, cfg.TokenTTL, now) || !csrfTokenMatch(token, got) {
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
	return newCSRFTokenAt(secret, time.Now())
}

func newCSRFTokenAt(secret []byte, now time.Time) string {
	nonce := make([]byte, 32)
	mustCSRFRand(nonce)
	n := hex.EncodeToString(nonce)
	payload := n + ":" + strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func csrfTokenValid(secret []byte, token string, ttl time.Duration, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[1])) != 1 {
		return false
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	// Verify timestamp if present: payload format is <nonce>:<unix_secs>
	subparts := strings.Split(parts[0], ":")
	if len(subparts) == 2 {
		ts, err := strconv.ParseInt(subparts[1], 10, 64)
		if err != nil {
			return false
		}
		tokenTime := time.Unix(ts, 0)
		if now.Sub(tokenTime) > ttl || tokenTime.After(now.Add(5*time.Minute)) {
			return false
		}
	}
	return true
}
