package context

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedirect_BlocksOpenRedirect(t *testing.T) {
	cases := []string{
		"https://evil.example",
		"http://evil.example/phish",
		"//evil.example",
		`/\evil.example`,
		"@evil.example",
		"login@evil.example",
		"relative",
		"javascript:alert(1)",
		"data:text/html,hi",
		"\r\nLocation: https://evil.example",
	}
	for _, loc := range cases {
		rec := httptest.NewRecorder()
		c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if err := c.Redirect(302, loc); err == nil {
			t.Fatalf("expected error for %q", loc)
		}
	}
}

func TestRedirect_AllowsRelative(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.Redirect(302, "/login"); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 302 || rec.Header().Get("Location") != "/login" {
		t.Fatalf("code=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectURL_AllowsAbsolute(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.RedirectURL(302, "https://example.com/ok"); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get("Location") != "https://example.com/ok" {
		t.Fatal(rec.Header().Get("Location"))
	}
}

func TestRedirectURL_BlocksCRLF(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.RedirectURL(302, "https://example.com/ok\r\nInjected: true"); err == nil {
		t.Fatal("expected error on CRLF in RedirectURL")
	}
}

func TestSanitizeDownloadFilename(t *testing.T) {
	got := sanitizeDownloadFilename("../../x\n\".bin")
	if got == "" || strings.ContainsAny(got, "\n\"\\") {
		t.Fatalf("got %q", got)
	}
}

func TestSafeUploadPath(t *testing.T) {
	got := SafeUploadPath("uploads", "../../etc/passwd")
	want := filepath.Join("uploads", "passwd")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
