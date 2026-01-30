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

	// Pre-convert to byte slice for performance
	keyBytes := []byte(config.SigningKey)

	return func(c *context.Context) error {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			return config.ErrorHandler(c, errors.New("missing auth header"))
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return config.ErrorHandler(c, errors.New("invalid auth header"))
		}

		tokenString := parts[1]
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
