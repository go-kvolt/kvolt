package middleware

import (
	"errors"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/golang-jwt/jwt/v5"
)

// JWTOptional validates a token when present. Missing token continues without claims.
// Invalid token fails with 401 unless SkipOnInvalid is true (then continues anonymously).
func JWTOptional(config JWTConfig) context.HandlerFunc {
	if config.KeyFunc == nil && strings.TrimSpace(config.SigningKey) == "" {
		panic("middleware: JWTOptional SigningKey or KeyFunc is required")
	}
	normalizeJWTConfig(&config)
	extractor := buildJWTExtractor(config.TokenLookup, config.AuthScheme)

	return func(c *context.Context) error {
		tokenString, err := extractor(c)
		if err != nil {
			// No token → anonymous continue
			c.Next()
			return nil
		}

		token, err := parseJWTToken(tokenString, config)
		if err != nil || token == nil || !token.Valid {
			if config.SkipOnInvalid {
				c.Next()
				return nil
			}
			return config.ErrorHandler(c, err)
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			if config.SkipOnInvalid {
				c.Next()
				return nil
			}
			return config.ErrorHandler(c, errors.New("invalid claims"))
		}
		c.Set(config.ContextKey, claims)
		c.Next()
		return nil
	}
}

// Claims returns JWT MapClaims previously stored by JWT / JWTOptional middleware.
func Claims(c *context.Context, contextKey ...string) (jwt.MapClaims, bool) {
	key := "user"
	if len(contextKey) > 0 && contextKey[0] != "" {
		key = contextKey[0]
	}
	v, ok := c.Get(key)
	if !ok {
		return nil, false
	}
	claims, ok := v.(jwt.MapClaims)
	return claims, ok
}

// ClaimString reads a string claim from JWT middleware storage.
func ClaimString(c *context.Context, claim string, contextKey ...string) string {
	claims, ok := Claims(c, contextKey...)
	if !ok {
		return ""
	}
	v, ok := claims[claim]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
