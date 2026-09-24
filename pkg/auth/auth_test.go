package auth

import (
	"testing"
	"time"
)

func TestGenerateTokenAndParseToken(t *testing.T) {
	SetSecret("test-secret")
	defer SetSecret("test-secret")

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
	SetSecret("test-secret")
	_, err := ParseToken("bad-token")
	if err == nil {
		t.Error("ParseToken bad format: want error")
	}
}

func TestParseToken_InvalidSignature(t *testing.T) {
	SetSecret("secret-a")
	token, err := GenerateToken(Claims{"x": "y"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	SetSecret("secret-b")
	_, err = ParseToken(token)
	if err == nil {
		t.Error("ParseToken wrong secret: want error")
	}
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
}

func TestGenerateToken_RequiresSecret(t *testing.T) {
	SetSecret("")
	_, err := GenerateToken(Claims{"a": 1}, time.Hour)
	if err == nil {
		t.Fatal("expected error without secret")
	}
	SetSecret("test-secret")
}
