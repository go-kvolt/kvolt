package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/golang-jwt/jwt/v5"
)

func TestTenant_HeaderRequired(t *testing.T) {
	mw := Tenant(TenantConfig{Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = mw(c)
	if w.Code != 400 {
		t.Fatalf("want 400 got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("X-Tenant-ID", "acme")
	c2 := context.New(w2, r2)
	hit := false
	c2.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		hit = true
		if TenantID(c) != "acme" {
			t.Fatalf("tenant=%q", TenantID(c))
		}
		return c.String(200, "ok")
	}}
	_ = mw(c2)
	if !hit || w2.Code != 200 {
		t.Fatalf("hit=%v code=%d", hit, w2.Code)
	}
}

func TestTenant_FromClaim(t *testing.T) {
	mw := Tenant(TenantConfig{Claim: "tid", Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Set("user", jwt.MapClaims{"tid": "t1"})
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error {
		if TenantID(c) != "t1" {
			t.Fatalf("got %q", TenantID(c))
		}
		return c.String(200, "ok")
	}}
	_ = mw(c)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestTenant_HeaderClaimMismatch(t *testing.T) {
	mw := Tenant(TenantConfig{Claim: "tid", Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "other")
	c := context.New(w, r)
	c.Set("user", jwt.MapClaims{"tid": "t1"})
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = mw(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d", w.Code)
	}
}

func TestTenant_ClaimIgnoresHeaderAlone(t *testing.T) {
	// When Claim is configured, a bare header must not satisfy Required.
	mw := Tenant(TenantConfig{Claim: "tid", Required: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Tenant-ID", "spoofed")
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = mw(c)
	if w.Code != 400 {
		t.Fatalf("want 400 got %d", w.Code)
	}
}

func TestRequirePermission(t *testing.T) {
	check := func(c *context.Context, perm string) (bool, error) {
		return perm == "inv.read", nil
	}
	okMW := RequirePermission("inv.read", check)
	denyMW := RequirePermission("inv.write", check)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = okMW(c)
	if w.Code != 200 {
		t.Fatalf("allow code=%d", w.Code)
	}

	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/", nil))
	c2.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = denyMW(c2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("deny want 403 got %d", w2.Code)
	}
}

func TestRequireAnyPermission(t *testing.T) {
	check := func(c *context.Context, perm string) (bool, error) {
		return perm == "b", nil
	}
	mw := RequireAnyPermission(check, "a", "b")
	w := httptest.NewRecorder()
	c := context.New(w, httptest.NewRequest("GET", "/", nil))
	c.Handlers = []context.HandlerFunc{func(c *context.Context) error { return c.String(200, "ok") }}
	_ = mw(c)
	if w.Code != 200 {
		t.Fatalf("code=%d", w.Code)
	}
}
