package context_test

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

type TestHeaderReq struct {
	AuthToken string `header:"Authorization"`
	APIKey    string `header:"x-api-key"`
	TenantID  string `header:"X-Tenant-ID"`
	RequestID string `header:"request-id"`
	UserAgent string
	Port      int `header:"X-Target-Port"`
}

func TestContext_BindHeader(t *testing.T) {
	// Nil context or nil request check
	var cNil *context.Context
	if err := cNil.BindHeader(&TestHeaderReq{}); err == nil {
		t.Fatal("expected error on nil context BindHeader")
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer my-secret-token")
	req.Header.Set("X-API-KEY", "apikey123")
	req.Header.Set("x-tenant-id", "tenant-erp-01")
	req.Header.Set("Request-Id", "req-xyz-99")
	req.Header.Set("UserAgent", "EnterpriseBot/1.0")
	req.Header.Set("X-Target-Port", "8080")

	rec := httptest.NewRecorder()
	c := context.New(rec, req)

	var h TestHeaderReq
	if err := c.BindHeader(&h); err != nil {
		t.Fatalf("unexpected BindHeader error: %v", err)
	}

	if h.AuthToken != "Bearer my-secret-token" {
		t.Errorf("Authorization mismatch: %q", h.AuthToken)
	}
	if h.APIKey != "apikey123" {
		t.Errorf("APIKey mismatch: %q", h.APIKey)
	}
	if h.TenantID != "tenant-erp-01" {
		t.Errorf("TenantID mismatch: %q", h.TenantID)
	}
	if h.RequestID != "req-xyz-99" {
		t.Errorf("RequestID mismatch: %q", h.RequestID)
	}
	if h.UserAgent != "EnterpriseBot/1.0" {
		t.Errorf("UserAgent mismatch: %q", h.UserAgent)
	}
	if h.Port != 8080 {
		t.Errorf("Port mismatch: %d", h.Port)
	}
}
