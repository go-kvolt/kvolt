package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
)

// BasicAuthConfig configures HTTP Basic authentication.
type BasicAuthConfig struct {
	// Users maps username -> password (plain). Prefer Validator for production.
	Users map[string]string
	// Validator returns true when credentials are valid.
	Validator func(user, pass string) bool
	// Realm default "Restricted".
	Realm string
	// ContextKey stores username when set (default "basic_user").
	ContextKey string
}

// BasicAuth protects routes with HTTP Basic Auth.
func BasicAuth(users map[string]string) func(c *context.Context) error {
	return BasicAuthWithConfig(BasicAuthConfig{Users: users})
}

// BasicAuthWithConfig returns configurable Basic Auth middleware.
func BasicAuthWithConfig(cfg BasicAuthConfig) func(c *context.Context) error {
	if cfg.Realm == "" {
		cfg.Realm = "Restricted"
	}
	if cfg.ContextKey == "" {
		cfg.ContextKey = "basic_user"
	}
	if cfg.Validator == nil {
		// Dummy secret keeps password hashing work similar for unknown usernames.
		const dummy = "kvolt-basic-auth-missing-user-dummy-value"
		cfg.Validator = func(user, pass string) bool {
			want, ok := cfg.Users[user]
			if !ok {
				want = dummy
			}
			a := sha256.Sum256([]byte(pass))
			b := sha256.Sum256([]byte(want))
			match := subtle.ConstantTimeCompare(a[:], b[:]) == 1
			if !ok {
				return false
			}
			return match
		}
	}

	return func(c *context.Context) error {
		if c == nil || c.Request == nil {
			return unauthorizedBasic(c, cfg.Realm)
		}
		auth := c.Request.Header.Get("Authorization")
		const basic = "Basic "
		if len(auth) <= len(basic) || !strings.EqualFold(auth[:len(basic)-1], "Basic") || auth[len(basic)-1] != ' ' {
			return unauthorizedBasic(c, cfg.Realm)
		}
		raw, err := base64.StdEncoding.DecodeString(auth[len(basic):])
		if err != nil {
			return unauthorizedBasic(c, cfg.Realm)
		}
		parts := strings.SplitN(string(raw), ":", 2)
		if len(parts) != 2 || !cfg.Validator(parts[0], parts[1]) {
			return unauthorizedBasic(c, cfg.Realm)
		}
		c.Set(cfg.ContextKey, parts[0])
		c.Next()
		return nil
	}
}

func unauthorizedBasic(c *context.Context, realm string) error {
	realm = sanitizeBasicRealm(realm)
	if c != nil && c.Writer != nil {
		c.Writer.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	}
	return httperr.Unauthorized("unauthorized", "basic auth required")
}

func sanitizeBasicRealm(realm string) string {
	var b strings.Builder
	for _, r := range realm {
		if r < 32 || r == '"' || r == '\\' {
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	if out == "" {
		return "Restricted"
	}
	return out
}

// APIKeyConfig validates an API key from header or query.
type APIKeyConfig struct {
	// Keys accepted API keys (constant-time compared).
	Keys []string
	// Lookup e.g. "header:X-API-Key" or "query:api_key". Default header:X-API-Key.
	Lookup string
	// ContextKey stores the matched key (default "api_key").
	ContextKey string
}

// APIKey requires a matching API key.
func APIKey(keys ...string) func(c *context.Context) error {
	return APIKeyWithConfig(APIKeyConfig{Keys: keys})
}

// APIKeyWithConfig returns configurable API key middleware.
func APIKeyWithConfig(cfg APIKeyConfig) func(c *context.Context) error {
	if cfg.Lookup == "" {
		cfg.Lookup = "header:X-API-Key"
	}
	if cfg.ContextKey == "" {
		cfg.ContextKey = "api_key"
	}
	parts := strings.SplitN(cfg.Lookup, ":", 2)
	source, name := "header", "X-API-Key"
	if len(parts) == 2 {
		source, name = parts[0], parts[1]
	}

	return func(c *context.Context) error {
		if c == nil || c.Request == nil {
			return httperr.Unauthorized("unauthorized", "api key required")
		}
		var got string
		switch source {
		case "query":
			got = c.Query(name)
		default:
			got = c.Request.Header.Get(name)
		}
		if got == "" {
			return httperr.Unauthorized("unauthorized", "api key required")
		}
		gotHash := sha256.Sum256([]byte(got))
		ok := false
		for _, k := range cfg.Keys {
			kHash := sha256.Sum256([]byte(k))
			if subtle.ConstantTimeCompare(gotHash[:], kHash[:]) == 1 {
				ok = true
			}
		}
		if !ok {
			return httperr.Unauthorized("unauthorized", "invalid api key")
		}
		c.Set(cfg.ContextKey, got)
		c.Next()
		return nil
	}
}

var _ = http.StatusUnauthorized
