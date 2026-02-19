package middleware

import (
	"strings"

	"github.com/go-kvolt/kvolt/context"
	"github.com/go-kvolt/kvolt/pkg/session"
)

// SessionConfig defines configuration for Session middleware.
type SessionConfig struct {
	// Manager is the session manager instance.
	Manager *session.Manager
	// Lookup is a string in the form of "source:key".
	// Supported: "cookie:name", "header:name", "query:name".
	// Default: "cookie:session_id"
	Lookup string
	// ContextKey is the key to store session data in context.
	// Default: "session"
	ContextKey string
}

// Session returns a middleware that validates sessions.
func Session(config SessionConfig) func(c *context.Context) error {
	if config.Lookup == "" {
		config.Lookup = "cookie:session_id"
	}
	if config.ContextKey == "" {
		config.ContextKey = "session"
	}

	return func(c *context.Context) error {
		token := extractToken(c, config.Lookup)
		if token == "" {
			return c.Status(401).String(401, "Unauthorized: No session token")
		}

		data, err := config.Manager.Get(token)
		if err != nil {
			return c.Status(401).String(401, "Unauthorized: Invalid session")
		}

		c.Set(config.ContextKey, data)
		c.Next()
		return nil
	}
}

func extractToken(c *context.Context, lookup string) string {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	source, key := parts[0], parts[1]

	switch source {
	case "cookie":
		cookie, err := c.Request.Cookie(key)
		if err == nil {
			return cookie.Value
		}
	case "header":
		return c.Request.Header.Get(key)
	case "query":
		return c.Request.URL.Query().Get(key)
	}
	return ""
}
