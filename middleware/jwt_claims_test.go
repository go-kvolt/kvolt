package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/golang-jwt/jwt/v5"
)

func TestJWTOptional_NoToken(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	called := false
	c.Handlers = []context.HandlerFunc{
		func(c *context.Context) error {
			called = true
			_, ok := Claims(c)
			if ok {
				t.Fatal("expected no claims")
			}
			return c.String(200, "ok")
		},
	}
	JWTOptional(JWTConfig{SigningKey: "secret"})(c)
	if !called || w.Code != 200 {
		t.Fatalf("called=%v code=%d", called, w.Code)
	}
}

func TestJWTOptional_ValidToken(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+s)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		func(c *context.Context) error {
			if ClaimString(c, "sub") != "u1" {
				t.Fatalf("claim sub=%q", ClaimString(c, "sub"))
			}
			return c.String(http.StatusOK, "ok")
		},
	}
	JWTOptional(JWTConfig{SigningKey: "secret"})(c)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestLimiterWithConfig_KeyFunc(t *testing.T) {
	lim := LimiterWithConfig(LimiterConfig{
		RPS:   1,
		Burst: 1,
		KeyFunc: func(c *context.Context) string {
			return "tenant-a"
		},
	})
	hits := 0
	handler := func(c *context.Context) error {
		hits++
		return c.String(200, "ok")
	}

	c1 := context.New(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	c1.Handlers = []context.HandlerFunc{handler}
	_ = lim(c1)

	rec2 := httptest.NewRecorder()
	c2 := context.New(rec2, httptest.NewRequest("GET", "/", nil))
	c2.Handlers = []context.HandlerFunc{handler}
	_ = lim(c2)

	if hits != 1 {
		t.Fatalf("expected second request blocked, hits=%d", hits)
	}
	if rec2.Code != 429 {
		t.Fatalf("want 429 got %d body=%s", rec2.Code, rec2.Body.String())
	}
}
