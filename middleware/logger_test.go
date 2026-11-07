package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
)

func TestSanitizeLogPath_StripsNewlines(t *testing.T) {
	got := sanitizeLogPath("/x\ninjected")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("newline survived: %q", got)
	}
	if got != "/xinjected" {
		t.Fatalf("got %q", got)
	}
}

func TestLogger_TextPathQuotedNoNewline(t *testing.T) {
	var logged string
	prev := emitLog
	emitLog = func(msg string) { logged = msg }
	t.Cleanup(func() { emitLog = prev })

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/ok", nil)
	// Simulate decoded %0A in URL.Path (net/http does this for clients).
	r.URL.Path = "/ok\nFAKE"
	c := context.New(w, r)
	c.Handlers = []context.HandlerFunc{
		LoggerWithConfig(LoggerConfig{Format: "text"}),
		func(c *context.Context) error { return c.String(200, "ok") },
	}
	c.Next()

	if logged == "" {
		t.Fatal("no log line")
	}
	// Raw newline would forge a second log line; sanitized+quoted keeps one line.
	if strings.Count(logged, "\n") != 1 || !strings.HasSuffix(logged, "\n") {
		t.Fatalf("forged/multiline log: %q", logged)
	}
	if strings.Contains(logged, "\nFAKE") {
		t.Fatalf("path newline forged log content: %q", logged)
	}
}
