package auth

import (
	"testing"
	"time"
)

func TestGenerateTokenAndParseToken(t *testing.T) {
	SetSecret("test-secret")
	defer SetSecret("kvolt-default-secret")

	token, err := GenerateToken(Claims{"user": "alice", "role": "admin"}, time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken: token empty")
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims["user"] != "alice" || claims["role"] != "admin" {
		t.Errorf("ParseToken: claims %v", claims)
	}
}

func TestParseToken_InvalidFormat(t *testing.T) {
	_, err := ParseToken("bad-token")
	if err == nil {
		t.Error("ParseToken bad format: want error")
	}
}

func TestParseToken_InvalidSignature(t *testing.T) {
	SetSecret("secret-a")
	token, _ := GenerateToken(Claims{"x": "y"}, time.Hour)
	SetSecret("secret-b")
	_, err := ParseToken(token)
	if err == nil {
		t.Error("ParseToken wrong secret: want error")
	}
	SetSecret("kvolt-default-secret")
}

func TestParseToken_Expired(t *testing.T) {
	SetSecret("test-secret")
	token, err := GenerateToken(Claims{"user": "bob"}, -time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	_, err = ParseToken(token)
	if err == nil {
		t.Error("ParseToken expired: want error")
	}
	SetSecret("kvolt-default-secret")
}
