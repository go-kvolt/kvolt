package swagger

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kvolt/kvolt"
	"github.com/go-kvolt/kvolt/context"
	"github.com/swaggo/swag"
)

// ReadDoc is a helper to read the generated Swagger documentation.
// It wraps swaggo/swag.ReadDoc() so users don't need to import swag directly.
func ReadDoc() (string, error) {
	return swag.ReadDoc()
}

// Adapter returns a RouteProvider that wraps a KVolt engine.
func Adapter(e *kvolt.Engine) RouteProvider {
	return &kvoltAdapter{engine: e}
}

type kvoltAdapter struct {
	engine *kvolt.Engine
}

func (a *kvoltAdapter) Routes() []RouteInfo {
	kvoltRoutes := a.engine.Routes()
	var routes []RouteInfo
	for _, r := range kvoltRoutes {
		routes = append(routes, RouteInfo{
			Method:  r.Method,
			Path:    r.Path,
			Summary: r.Summary,
		})
	}
	return routes
}

// RouteProvider interface allows decoupling from direct kvolt.Engine dependency if needed,
// but for simplicity we can define a struct that matches kvolt.RouteInfo
type RouteProvider interface {
	Routes() []RouteInfo
}

// Config configures the Swagger UI.
type Config struct {
	// SpecJSON is the raw OpenAPI/Swagger JSON string.
	// If empty, and App is provided, it will generate a basic spec.
	SpecJSON string
	// Title is the page title. Default: "KVolt API Docs"
	Title string
	// App is the KVolt engine instance for auto-discovery
	// We use an interface to avoid circular import if kvolt imports swagger.
	// However, usually user imports both.
	// But kvolt package already exists. Let's try to import kvolt if possible.
	// Wait, if we import "github.com/go-kvolt/kvolt" here, and user uses it, it works.
	// But if kvolt package itself imported swagger, it would be circular.
	// KVolt core does NOT import swagger. So we are safe.
	// But to be generic, let's use a closure or interface.
	// RoutesProvider is the interface for auto-discovery
	RoutesProvider interface {
		Routes() []RouteInfo
	}
	// Disabled, if true, disables the documentation endpoint (returns 404).
	Disabled bool

	// Host is the address where the server is running (e.g. "localhost:8080").
	// Used for logging the full URL on startup.
	Host string
}

// RouteInfo must match kvolt.RouteInfo
type RouteInfo struct {
	Method  string
	Path    string
	Summary string
}

// Handler returns a KVolt handler that serves the Swagger UI and the spec.
// Mount this at a path like "/swagger/*any".
func Handler(cfg Config) func(c *context.Context) error {
	if cfg.Title == "" {
		cfg.Title = "KVolt API Docs"
	}

	// Lazy generation of spec
	var cachedSpec []byte

	if !cfg.Disabled {
		baseURL := cfg.Host
		if baseURL == "" {
			baseURL = "localhost" // Fallback
		}
		// If Host doesn't contain scheme, assume http://
		if !strings.HasPrefix(baseURL, "http") {
			baseURL = "http://" + baseURL
		}
		// We can't know the mount path (e.g. /swagger/*any) inside the handler factory easily
		// unless passed, but user usually mounts at /swagger.
		fmt.Printf("📖 API Documentation enabled: %s/swagger/index.html\n", baseURL)
	}

	return func(c *context.Context) error {
		if cfg.Disabled {
			return c.Status(404).String(404, "Not Found")
		}

		path := c.Param("any")

		// 1. Serve the Spec JSON
		if strings.HasSuffix(path, "doc.json") {
			c.Writer.Header().Set("Content-Type", "application/json")
			c.Writer.WriteHeader(200)

			if len(cfg.SpecJSON) > 0 {
				_, err := c.Writer.Write([]byte(cfg.SpecJSON))
				return err
			}

			// Generate if not provided
			if len(cachedSpec) == 0 && cfg.RoutesProvider != nil {
				routes := cfg.RoutesProvider.Routes()
				cachedSpec = generateOpenAPI(routes, cfg.Title)
			}

			if len(cachedSpec) > 0 {
				_, err := c.Writer.Write(cachedSpec)
				return err
			}

			// Fallback empty spec
			return c.String(200, "{}")
		}

		// 2. Serve the HTML UI
		c.Writer.Header().Set("Content-Type", "text/html")
		html := fmt.Sprintf(uiTemplate, cfg.Title)
		return c.HTML(200, html)
	}
}

func generateOpenAPI(routes []RouteInfo, title string) []byte {
	paths := make(map[string]map[string]interface{})

	for _, r := range routes {
		if r.Path == "" {
			continue
		}
		// Convert /:id to {id} for OpenAPI?
		// KVolt uses :id. Swagger uses {id}.
		// Simple replacement for now.
		openAPIPath := r.Path
		segments := strings.Split(r.Path, "/")
		for i, seg := range segments {
			if strings.HasPrefix(seg, ":") {
				segments[i] = "{" + seg[1:] + "}"
			} else if strings.HasPrefix(seg, "*") {
				segments[i] = "{" + seg[1:] + "}"
			}
		}
		openAPIPath = strings.Join(segments, "/")

		if paths[openAPIPath] == nil {
			paths[openAPIPath] = make(map[string]interface{})
		}

		method := strings.ToLower(r.Method)
		summary := r.Summary
		if summary == "" {
			summary = fmt.Sprintf("%s %s", r.Method, r.Path)
		}
		paths[openAPIPath][method] = map[string]interface{}{
			"summary": summary,
			"responses": map[string]interface{}{
				"200": map[string]string{"description": "OK"},
			},
		}
	}

	spec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":   title,
			"version": "1.0.0",
		},
		"paths": paths,
	}

	b, _ := json.Marshal(spec)
	return b
}

const uiTemplate = `<!doctype html>
<html>
  <head>
    <title>%s</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
      body {
        margin: 0;
      }
    </style>
  </head>
  <body>
    <script
      id="api-reference"
      data-url="doc.json"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>
`
