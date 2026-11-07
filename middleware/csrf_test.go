package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestCSRF_GetSetsCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := context.New(rec, req)
	c.Handlers = []context.HandlerFunc{
		CSRF(),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	found := false
	for _, ck := range cookies {
		if ck.Name == "csrf_token" && ck.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected csrf_token cookie")
	}
}

func TestCSRF_PostWithoutTokenForbidden(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := context.New(rec, req)
	c.Handlers = []context.HandlerFunc{
		CSRF(),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body %s", rec.Code, rec.Body.String())
	}
}

func TestCSRF_PostWithMatchingToken(t *testing.T) {
	secret := "test-csrf-secret"
	token := newCSRFToken([]byte(secret))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	rec := httptest.NewRecorder()
	c := context.New(rec, req)
	c.Handlers = []context.HandlerFunc{
		CSRFWithConfig(CSRFConfig{Secret: secret}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if rec.Code != 200 {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestCSRF_RejectsUnsignedPlantedCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "planted.notreal"})
	req.Header.Set("X-CSRF-Token", "planted.notreal")
	rec := httptest.NewRecorder()
	c := context.New(rec, req)
	c.Handlers = []context.HandlerFunc{
		CSRFWithConfig(CSRFConfig{Secret: "test-csrf-secret"}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestCSRF_EmptySkipPathIgnored(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/form", nil)
	rec := httptest.NewRecorder()
	c := context.New(rec, req)
	c.Handlers = []context.HandlerFunc{
		CSRFWithConfig(CSRFConfig{SkipPaths: []string{"", "  "}}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty SkipPaths must not disable CSRF; got %d", rec.Code)
	}
}
