package middleware

import (
	"errors"
	"strings"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/pkg/httperr"
	"github.com/golang-jwt/jwt/v5"
)

func httperrUnauthorized() error {
	return httperr.Unauthorized("unauthorized", "Unauthorized")
}

// JWTConfig defines the config for JWT middleware.
type JWTConfig struct {
	// SigningKey is the HMAC secret (Required unless KeyFunc is set).
	SigningKey string

	// KeyFunc supplies the verification key (e.g. RSA public key for RS256).
	// When set, SigningKey may be empty. Prefer KeyFunc for asymmetric algorithms.
	KeyFunc jwt.Keyfunc

	// SigningMethod restricts accepted algorithms (default "HS256").
	// Use "RS256" with KeyFunc returning an *rsa.PublicKey.
	SigningMethod string

	// ContextKey is the key used to store claims in context (Default: "user").
	ContextKey string

	// TokenLookup is a string in the form of "<source>:<name>" that is used
	// to extract token from the request.
	// Optional. Default value "header:Authorization".
	// Possible values:
	// - "header:<name>"
	// - "cookie:<name>"
	// - "query:<name>" (discouraged: tokens leak via logs, Referer, and history)
	TokenLookup string

	// AuthScheme to be used in the Authorization header.
	// Optional. Default value "Bearer". Matching is case-insensitive.
	AuthScheme string

	// Issuer when non-empty requires claim "iss" to match.
	Issuer string
	// Audience when non-empty requires claim "aud" to include this value.
	Audience string
	// RequireExp when true (default) rejects tokens without an exp claim.
	RequireExp *bool

	// SkipOnInvalid (JWTOptional only): treat invalid tokens as anonymous instead of 401.
	SkipOnInvalid bool

	// ErrorHandler handles errors during token validation (Optional).
	ErrorHandler func(c *context.Context, err error) error
}

func jwtRequireExp(cfg JWTConfig) bool {
	if cfg.RequireExp == nil {
		return true
	}
	return *cfg.RequireExp
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
		scheme := authScheme
		return func(c *context.Context) (string, error) {
			auth := c.Request.Header.Get(name)
			if auth == "" {
				return "", errors.New("missing auth header")
			}
			if scheme != "" {
				if len(auth) <= len(scheme) || auth[len(scheme)] != ' ' {
					return "", errors.New("invalid auth header format")
				}
				if !strings.EqualFold(auth[:len(scheme)], scheme) {
					return "", errors.New("invalid auth header format")
				}
				return auth[len(scheme)+1:], nil
			}
			return auth, nil
		}
	case "query":
		// Discouraged: tokens in query strings leak via logs, Referer, and browser history.
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

func parseJWTToken(tokenString string, cfg JWTConfig) (*jwt.Token, error) {
	methodName := cfg.SigningMethod
	if methodName == "" {
		methodName = "HS256"
	}
	keyBytes := []byte(cfg.SigningKey)
	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != methodName {
				return nil, errors.New("unexpected signing method")
			}
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return keyBytes, nil
		}
	} else {
		inner := keyFunc
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if token.Method == nil || token.Method.Alg() != methodName {
				return nil, errors.New("unexpected signing method")
			}
			return inner(token)
		}
	}

	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{methodName})}
	if jwtRequireExp(cfg) {
		opts = append(opts, jwt.WithExpirationRequired())
	}
	if cfg.Issuer != "" {
		opts = append(opts, jwt.WithIssuer(cfg.Issuer))
	}
	if cfg.Audience != "" {
		opts = append(opts, jwt.WithAudience(cfg.Audience))
	}
	parser := jwt.NewParser(opts...)
	return parser.Parse(tokenString, keyFunc)
}

func normalizeJWTConfig(config *JWTConfig) {
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(c *context.Context, err error) error {
			return httperrUnauthorized()
		}
	}
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.AuthScheme == "" && strings.HasPrefix(config.TokenLookup, "header:") {
		config.AuthScheme = "Bearer"
	}
	if config.SigningMethod == "" {
		config.SigningMethod = "HS256"
	}
}

// JWT returns a JWT authentication middleware.
func JWT(config JWTConfig) context.HandlerFunc {
	if config.KeyFunc == nil && strings.TrimSpace(config.SigningKey) == "" {
		panic("middleware: JWT SigningKey or KeyFunc is required")
	}
	normalizeJWTConfig(&config)
	extractor := buildJWTExtractor(config.TokenLookup, config.AuthScheme)

	return func(c *context.Context) error {
		tokenString, err := extractor(c)
		if err != nil {
			return config.ErrorHandler(c, err)
		}

		token, err := parseJWTToken(tokenString, config)
		if err != nil || token == nil || !token.Valid {
			return config.ErrorHandler(c, err)
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return config.ErrorHandler(c, errors.New("invalid claims"))
		}

		c.Set(config.ContextKey, claims)
		c.Next()
		return nil
	}
}
