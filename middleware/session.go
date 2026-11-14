package middleware

import (
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/go-kvolt/kvolt/v2/pkg/session"
)

// SessionConfig defines configuration for Session middleware.
type SessionConfig struct {
	Manager    *session.Manager
	Lookup     string
	ContextKey string
}

// Session returns a middleware that validates sessions.
func Session(config SessionConfig) func(c *context.Context) error {
	if config.Manager == nil {
		panic("middleware: session Manager is required")
	}
	if config.Lookup == "" {
		config.Lookup = "cookie:session_id"
	}
	if config.ContextKey == "" {
		config.ContextKey = "session"
	}

	return func(c *context.Context) error {
		if c == nil {
			return nil
		}
		token := extractToken(c, config.Lookup)
		if token == "" {
			return httperr.Unauthorized("unauthorized", "no session token")
		}

		data, err := config.Manager.Get(token)
		if err != nil {
			return httperr.Unauthorized("unauthorized", "invalid session")
		}

		c.Set(config.ContextKey, data)
		c.Next()
		return nil
	}
}

func extractToken(c *context.Context, lookup string) string {
	if c == nil || c.Request == nil {
		return ""
	}
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	source, key := parts[0], parts[1]

	switch source {
	case "cookie":
		cookie, err := c.Request.Cookie(key)
		if err == nil {
			return strings.TrimSpace(cookie.Value)
		}
	case "header":
		val := strings.TrimSpace(c.Request.Header.Get(key))
		if strings.EqualFold(key, "authorization") && len(val) > 7 && strings.EqualFold(val[:7], "bearer ") {
			val = strings.TrimSpace(val[7:])
		}
		return val
	case "query":
		// Discouraged: session tokens in query strings leak via logs/Referer/history.
		if c.Request.URL == nil {
			return ""
		}
		return strings.TrimSpace(c.Request.URL.Query().Get(key))
	}
	return ""
}
