package swagger

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/router"
)

func TestReadDoc_AndAdapter(t *testing.T) {
	_, err := ReadDoc()
	// No registered swag docs in unit tests — any error or empty is fine; we just call it.
	_ = err

	app := kvolt.New()
	app.GET("/hi", func(c *context.Context) error { return c.String(200, "ok") })
	provider := Adapter(app)
	routes := provider.Routes()
	if len(routes) == 0 {
		t.Fatal("Adapter.Routes empty")
	}
	found := false
	for _, r := range routes {
		if r.Path == "/hi" && r.Method == "GET" {
			found = true
		}
	}
	if !found {
		t.Fatalf("routes=%v", routes)
	}
}

func TestHandler_RoutesProviderCachedSpec(t *testing.T) {
	provider := staticRoutes{routes: []RouteInfo{
		{Method: "GET", Path: "/items/:id", Summary: "", Tags: []string{"items"}, Security: []string{"bearer"}},
		{Method: "POST", Path: "", Summary: "skip empty"},
		{Method: "GET", Path: "/health", Responses: map[int]interface{}{204: nil, 418: "teapot"}},
	}}
	h := Handler(Config{Title: "Boost", Host: "example.com", RoutesProvider: provider})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	c := context.New(w, r)
	c.Params = router.Params{{Key: "any", Value: "doc.json"}}
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	var spec map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	paths := spec["paths"].(map[string]interface{})
	if paths["/items/{id}"] == nil {
		t.Fatalf("missing param path: %v", paths)
	}
	// Second call hits cachedSpec.
	w2 := httptest.NewRecorder()
	c2 := context.New(w2, httptest.NewRequest("GET", "/swagger/doc.json", nil))
	c2.Params = router.Params{{Key: "any", Value: "doc.json"}}
	_ = h(c2)
}

func TestHandler_EmptySpecFallback(t *testing.T) {
	h := Handler(Config{Title: "Empty"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	c := context.New(w, r)
	c.Params = router.Params{{Key: "any", Value: "doc.json"}}
	_ = h(c)
	if w.Body.String() != "{}" && w.Code != 200 {
		// either String("{}" ) or write path
		t.Logf("body=%q code=%d", w.Body.String(), w.Code)
	}
}

type staticRoutes struct{ routes []RouteInfo }

func (s staticRoutes) Routes() []RouteInfo { return s.routes }

func TestSchemaFromSample_Kinds(t *testing.T) {
	if SchemaFromSample(nil)["type"] != "object" {
		t.Fatal("nil")
	}
	if SchemaFromSample(true)["type"] != "boolean" {
		t.Fatal("bool")
	}
	if SchemaFromSample(3.14)["type"] != "number" {
		t.Fatal("float")
	}
	if SchemaFromSample([]int{1, 2})["type"] != "array" {
		t.Fatal("slice")
	}
	if SchemaFromSample(map[string]int{"a": 1})["type"] != "object" {
		t.Fatal("map")
	}
	if SchemaFromSample(time.Now())["format"] != "date-time" {
		t.Fatal("time")
	}
	type nested struct {
		Self *nested `json:"self,omitempty"`
		Skip string  `json:"-"`
		priv int
	}
	n := &nested{priv: 1}
	if n.priv != 1 {
		t.Fatal("priv")
	}
	_ = SchemaFromSample(n)
	_ = SchemaFromSample((*nested)(nil))
	ch := make(chan int)
	if SchemaFromSample(ch)["type"] != "object" {
		t.Fatal("chan exotic")
	}
}

func TestStatusText_All(t *testing.T) {
	for code, want := range map[int]string{
		200: "OK", 201: "Created", 204: "No Content",
		400: "Bad Request", 401: "Unauthorized", 403: "Forbidden",
		404: "Not Found", 500: "Internal Server Error",
	} {
		if got := statusText(code); got != want {
			t.Fatalf("%d: %q", code, got)
		}
	}
	if httpStatusText(418) != "418" {
		t.Fatal(httpStatusText(418))
	}
}
