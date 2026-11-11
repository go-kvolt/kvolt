package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseToken_NoSecret(t *testing.T) {
	prev := secretKey
	prevSet := secretSet
	defer func() {
		mu.Lock()
		secretKey = prev
		secretSet = prevSet
		mu.Unlock()
	}()

	SetSecret("")
	if _, err := ParseToken("x.y.z"); err == nil {
		t.Fatal("want error without secret")
	}
}

func TestParseToken_WrongAlg(t *testing.T) {
	SetSecret("test-secret")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS384, jwt.MapClaims{
		"sub": "u",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := tok.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseToken(s); err == nil {
		t.Fatal("want reject non-HS256")
	}
}

func TestParseToken_NoneAlgRejected(t *testing.T) {
	SetSecret("test-secret")
	// Header alg=none is rejected by WithValidMethods / parser.
	raw := "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ1IiwiZXhwIjo0MTAyNDQ0ODAwfQ."
	if _, err := ParseToken(raw); err == nil {
		t.Fatal("want reject alg=none")
	}
}

func TestParseToken_EmptyString(t *testing.T) {
	SetSecret("test-secret")
	if _, err := ParseToken(""); err == nil {
		t.Fatal("want error")
	}
}

type nonHMACHS256 struct{}

func (nonHMACHS256) Alg() string                              { return "HS256" }
func (nonHMACHS256) Sign(string, interface{}) ([]byte, error) { return []byte("sig"), nil }
func (nonHMACHS256) Verify(string, []byte, interface{}) error { return nil }

func TestParseToken_UnexpectedMethodViaCustomAlg(t *testing.T) {
	jwt.RegisterSigningMethod("HS256", func() jwt.SigningMethod { return nonHMACHS256{} })
	defer jwt.RegisterSigningMethod("HS256", func() jwt.SigningMethod { return jwt.SigningMethodHS256 })

	SetSecret("test-secret")
	raw := "eyJhbGciOiJIUzI1NiJ9.eyJleHAiOjQxMDI0NDQ4MDB9.c2ln"
	if _, err := ParseToken(raw); err == nil {
		t.Fatal("want unexpected method error")
	}
}

func TestLoadSecretFromEnv(t *testing.T) {
	prev := secretKey
	prevSet := secretSet
	defer func() {
		mu.Lock()
		secretKey = prev
		secretSet = prevSet
		mu.Unlock()
	}()

	t.Setenv("JWT_SECRET", "env-secret-value")
	loadSecretFromEnv()
	if !secretSet || string(secretKey) != "env-secret-value" {
		t.Fatalf("set=%v key=%q", secretSet, secretKey)
	}
	tok, err := GenerateToken(Claims{"ok": true}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseToken(tok); err != nil {
		t.Fatal(err)
	}
}

func TestClaimsFromToken_Edges(t *testing.T) {
	if _, err := claimsFromToken(&jwt.Token{Valid: false}); err == nil {
		t.Fatal("want invalid token")
	}
	if _, err := claimsFromToken(&jwt.Token{Valid: true, Claims: jwt.RegisteredClaims{}}); err == nil {
		t.Fatal("want invalid claims")
	}
	got, err := claimsFromToken(&jwt.Token{Valid: true, Claims: jwt.MapClaims{"a": "b"}})
	if err != nil || got["a"] != "b" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
