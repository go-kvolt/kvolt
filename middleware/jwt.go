package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-kvolt/kvolt/context"
	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig defines the config for JWT middleware.
type JWTConfig struct {
	// SigningKey is the secret used to sign the JWT (Required).
	SigningKey string

	// ContextKey is the key used to store claims in context (Default: "user").
	ContextKey string

	// TokenLookup is a string in the form of "<source>:<name>" that is used
	// to extract token from the request.
	// Optional. Default value "header:Authorization".
	// Possible values:
	// - "header:<name>"
	// - "query:<name>"
	// - "cookie:<name>"
	TokenLookup string

	// AuthScheme to be used in the Authorization header.
	// Optional. Default value "Bearer".
	AuthScheme string

	// ErrorHandler handles errors during token validation (Optional).
	ErrorHandler func(c *context.Context, err error) error
}

// jwtExtractor extracts the token string from the request.
type jwtExtractor func(c *context.Context) (string, error)

func buildJWTExtractor(lookup, authScheme string) jwtExtractor {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		return func(c *context.Context) (string, error) {
			return "", errors.New("invalid token lookup config")
		}
	}
	source, name := parts[0], parts[1]
	switch source {
	case "header":
		lenAuth := len(authScheme)
		return func(c *context.Context) (string, error) {
			auth := c.Request.Header.Get(name)
			if auth == "" {
				return "", errors.New("missing auth header")
			}
			if lenAuth > 0 {
				if len(auth) > lenAuth+1 && auth[:lenAuth] == authScheme && auth[lenAuth] == ' ' {
					return auth[lenAuth+1:], nil
				}
				return "", errors.New("invalid auth header format")
			}
			return auth, nil
		}
	case "query":
		return func(c *context.Context) (string, error) {
			token := c.Request.URL.Query().Get(name)
			if token == "" {
				return "", errors.New("missing token in query")
			}
			return token, nil
		}
	case "cookie":
		return func(c *context.Context) (string, error) {
			cookie, err := c.Request.Cookie(name)
			if err != nil {
				return "", errors.New("missing token cookie")
			}
			return cookie.Value, nil
		}
	default:
		return func(c *context.Context) (string, error) {
			return "", errors.New("invalid token lookup config")
		}
	}
}

// JWT returns a JWT authentication middleware.
func JWT(config JWTConfig) context.HandlerFunc {
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(c *context.Context, err error) error {
			return c.Status(http.StatusUnauthorized).JSON(http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized",
			})
		}
	}
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.AuthScheme == "" && strings.HasPrefix(config.TokenLookup, "header:") {
		config.AuthScheme = "Bearer"
	}

	extractor := buildJWTExtractor(config.TokenLookup, config.AuthScheme)
	keyBytes := []byte(config.SigningKey)

	return func(c *context.Context) error {
		tokenString, err := extractor(c)
		if err != nil {
			return config.ErrorHandler(c, err)
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return keyBytes, nil
		})

		if err != nil || !token.Valid {
			return config.ErrorHandler(c, err)
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return config.ErrorHandler(c, errors.New("invalid claims"))
		}

		// Store claims in context (Zero alloc)
		c.Set(config.ContextKey, claims)

		c.Next()
		return nil
	}
}
