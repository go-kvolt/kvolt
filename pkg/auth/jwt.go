package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

// Token helpers (Simplified JWT implementation to avoid heavy dependency for now)
// Header.Payload.Signature

var (
	// secretKey is the signing key.
	// It loads from JWT_SECRET env var, or defaults to "kvolt-default-secret" with a warning.
	secretKey = []byte("kvolt-default-secret")
)

func init() {
	if env := os.Getenv("JWT_SECRET"); env != "" {
		secretKey = []byte(env)
	} else {
		// In a real logger we'd use pkg/logger, but standard log is safer for init() to avoid cycles/setup issues
		// fmt.Println("[WARNING] JWT_SECRET not set. Using default insecure secret.")
	}
}

// SetSecret allows setting the JWT secret programmatically.
func SetSecret(s string) {
	secretKey = []byte(s)
}

type Claims map[string]interface{}

func GenerateToken(claims Claims, expiry time.Duration) (string, error) {
	claims["exp"] = time.Now().Add(expiry).Unix()

	header := map[string]string{"alg": "HS256", "typ": "JWT"}

	hJSON, _ := json.Marshal(header)
	cJSON, _ := json.Marshal(claims)

	token := base64.RawURLEncoding.EncodeToString(hJSON) + "." + base64.RawURLEncoding.EncodeToString(cJSON)

	sig := sign(token, secretKey)
	return token + "." + sig, nil
}

func ParseToken(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// Verify Signature
	if !verify(parts[0]+"."+parts[1], parts[2], secretKey) {
		return nil, errors.New("invalid signature")
	}

	// Parse Claims
	var claims Claims
	cJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(cJSON, &claims); err != nil {
		return nil, err
	}

	// Check Exp
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, errors.New("token expired")
		}
	}

	return claims, nil
}

func sign(data string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func verify(data, sig string, secret []byte) bool {
	expected := sign(data, secret)
	return hmac.Equal([]byte(sig), []byte(expected))
}
