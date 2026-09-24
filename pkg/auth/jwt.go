package auth

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token helpers backed by github.com/golang-jwt/jwt/v5 (same stack as middleware.JWT).

var (
	mu        sync.RWMutex
	secretKey []byte
	secretSet bool
)

func init() {
	if env := os.Getenv("JWT_SECRET"); env != "" {
		secretKey = []byte(env)
		secretSet = true
	}
}

// SetSecret sets the HMAC signing secret. Required before GenerateToken/ParseToken
// unless JWT_SECRET is set in the environment.
func SetSecret(s string) {
	mu.Lock()
	defer mu.Unlock()
	secretKey = []byte(s)
	secretSet = s != ""
}

func currentSecret() ([]byte, error) {
	mu.RLock()
	defer mu.RUnlock()
	if !secretSet || len(secretKey) == 0 {
		return nil, errors.New("auth: JWT secret not configured; call auth.SetSecret or set JWT_SECRET")
	}
	return secretKey, nil
}

// Claims is a convenience map for HS256 tokens.
type Claims map[string]interface{}

// GenerateToken creates a signed HS256 JWT.
func GenerateToken(claims Claims, expiry time.Duration) (string, error) {
	secret, err := currentSecret()
	if err != nil {
		return "", err
	}
	mapClaims := jwt.MapClaims{}
	for k, v := range claims {
		mapClaims[k] = v
	}
	mapClaims["exp"] = time.Now().Add(expiry).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	return token.SignedString(secret)
}

// ParseToken validates an HS256 JWT and returns claims.
func ParseToken(tokenString string) (Claims, error) {
	secret, err := currentSecret()
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	out := Claims{}
	for k, v := range mc {
		out[k] = v
	}
	return out, nil
}
