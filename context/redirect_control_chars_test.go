package context

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Browsers strip tab / CR / LF while parsing a URL, so "/\t/evil.example" is followed
// as the protocol-relative "//evil.example". Every ASCII control character is rejected.
func TestRedirect_BlocksControlCharacters(t *testing.T) {
	cases := []string{
		"/\t/evil.example",
		"\t/\t/evil.example",
		"/\n/evil.example",
		"/\v/evil.example",
		"/\f/evil.example",
		"/ok\x7f",
		"/ok\x01path",
	}
	for _, loc := range cases {
		rec := httptest.NewRecorder()
		c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if err := c.Redirect(http.StatusFound, loc); err == nil {
			t.Fatalf("expected error for %q, got Location %q", loc, rec.Header().Get("Location"))
		}
		if got := rec.Header().Get("Location"); got != "" {
			t.Fatalf("Location must not be set for %q, got %q", loc, got)
		}
	}
}

// Surrounding whitespace is trimmed before validation, and the trimmed value is what is sent.
func TestRedirect_SendsTrimmedLocation(t *testing.T) {
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.Redirect(http.StatusFound, "  /dashboard?tab=1  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rec.Header().Get("Location"); got != "/dashboard?tab=1" {
		t.Fatalf("Location = %q, want trimmed path", got)
	}
}

func TestRedirectURL_BlocksControlCharacters(t *testing.T) {
	for _, loc := range []string{"https://ok.example/\tx", "https://ok.example/\x00", "https://ok.example/\x7f"} {
		rec := httptest.NewRecorder()
		c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if err := c.RedirectURL(http.StatusFound, loc); err == nil {
			t.Fatalf("expected error for %q", loc)
		}
	}
	rec := httptest.NewRecorder()
	c := New(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if err := c.RedirectURL(http.StatusFound, "https://ok.example/path"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
