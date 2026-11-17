package swagger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/router"
)

func docsPage(t *testing.T, cfg Config) string {
	t.Helper()
	rec := httptest.NewRecorder()
	c := context.New(rec, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	c.Params = router.Params{{Key: "any", Value: "index.html"}}
	if err := Handler(cfg)(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func scriptTag(url, integrity string) string {
	if integrity == "" {
		return `<script src="` + url + `"></script>`
	}
	return `<script src="` + url + `" integrity="` + integrity + `" crossorigin="anonymous"></script>`
}

// By default the page loads a pinned release and the browser checks its hash.
func TestHandler_DefaultScriptIsPinnedWithIntegrity(t *testing.T) {
	want := scriptTag(DefaultScriptURL, DefaultScriptIntegrity)
	if page := docsPage(t, Config{}); !strings.Contains(page, want) {
		t.Fatalf("default script tag missing, want %s in:\n%s", want, page)
	}
	if !strings.Contains(DefaultScriptURL, "@scalar/api-reference@") {
		t.Fatalf("DefaultScriptURL must name an exact version: %s", DefaultScriptURL)
	}
	if !strings.HasPrefix(DefaultScriptIntegrity, "sha384-") {
		t.Fatalf("DefaultScriptIntegrity must be a sha384 hash: %s", DefaultScriptIntegrity)
	}
	// An integrity value without a ScriptURL is ignored: it would not match the default script.
	if page := docsPage(t, Config{ScriptIntegrity: "sha384-other"}); !strings.Contains(page, want) {
		t.Fatalf("default integrity was overridden:\n%s", page)
	}
}

func TestHandler_CustomScriptURL(t *testing.T) {
	const custom = "/assets/scalar-1.2.3.js"

	// A self-hosted script replaces the default and carries no hash unless one is given.
	page := docsPage(t, Config{ScriptURL: custom})
	if !strings.Contains(page, scriptTag(custom, "")) {
		t.Fatalf("custom script tag missing:\n%s", page)
	}
	if strings.Contains(page, DefaultScriptURL) || strings.Contains(page, "integrity=") {
		t.Fatalf("default script or hash still present with a custom ScriptURL:\n%s", page)
	}

	page = docsPage(t, Config{ScriptURL: custom, ScriptIntegrity: "sha384-abc"})
	if !strings.Contains(page, scriptTag(custom, "sha384-abc")) {
		t.Fatalf("custom integrity missing:\n%s", page)
	}
}

// Both values are escaped, so they cannot break out of their attributes.
func TestHandler_ScriptValuesAreEscaped(t *testing.T) {
	page := docsPage(t, Config{ScriptURL: `x"></script><script>alert(1)</script>`})
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Fatalf("ScriptURL was not escaped:\n%s", page)
	}
	page = docsPage(t, Config{ScriptURL: "/s.js", ScriptIntegrity: `x" onload="alert(1)`})
	if strings.Contains(page, `onload="alert(1)"`) {
		t.Fatalf("ScriptIntegrity was not escaped:\n%s", page)
	}
}
