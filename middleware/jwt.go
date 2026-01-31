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

// JWT returns a JWT authentication middleware.
func JWT(config JWTConfig) context.HandlerFunc {
	// Defaults
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

	// Pre-parse TokenLookup to avoid overhead on every request
	parts := strings.Split(config.TokenLookup, ":")
	extractor := func(c *context.Context) (string, error) {
		return "", errors.New("invalid token lookup config")
	}

	switch parts[0] {
	case "header":
		headerName := parts[1]
		authScheme := config.AuthScheme
		lenAuthScheme := len(authScheme)
		extractor = func(c *context.Context) (string, error) {
			auth := c.Request.Header.Get(headerName)
			if auth == "" {
				return "", errors.New("missing auth header")
			}
			if lenAuthScheme > 0 {
				if len(auth) > lenAuthScheme+1 && auth[:lenAuthScheme] == authScheme && auth[lenAuthScheme] == ' ' {
					return auth[lenAuthScheme+1:], nil
				}
				return "", errors.New("invalid auth header format")
			}
			return auth, nil
		}
	case "query":
		queryParam := parts[1]
		extractor = func(c *context.Context) (string, error) {
			token := c.Request.URL.Query().Get(queryParam)
			if token == "" {
				return "", errors.New("missing token in query")
			}
			return token, nil
		}
	case "cookie":
		cookieName := parts[1]
		extractor = func(c *context.Context) (string, error) {
			cookie, err := c.Request.Cookie(cookieName)
			if err != nil {
				return "", errors.New("missing token cookie")
			}
			return cookie.Value, nil
		}
	}

	// Pre-convert to byte slice for performance
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
