package swagger

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kvolt/kvolt/context"
)

// RouteProvider interface allows decoupling from direct kvolt.Engine dependency if needed,
// but for simplicity we can define a struct that matches kvolt.RouteInfo
type RouteProvider interface {
	Routes() []struct {
		Method string
		Path   string
	}
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

const uiTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
  <style>
    body { margin: 0; padding: 0; background: #ffffff; } /* Light background */
    
    /* Topbar Branding */
    .swagger-ui .topbar { background-color: #f8fafc; border-bottom: 2px solid #eab308; }
    .swagger-ui .topbar .link { display: none; } /* Hide default Swagger logo */
    
    /* KVolt Logo Replacement */
    .swagger-ui .topbar .wrapper::before {
        content: "⚡ KVolt API Docs";
        color: #eab308; /* Yellow/Gold accent */
        font-weight: bold;
        font-size: 24px;
        font-family: sans-serif;
    }

    /* Button Colors overrides for brand consistency */
    .swagger-ui .btn.authorize { color: #eab308; border-color: #eab308; background: transparent; }
    .swagger-ui .btn.authorize svg { fill: #eab308; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: 'doc.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "StandaloneLayout",
        plugins: [
            function() {
                return {
                    components: {
                        Topbar: function() { return null; } // Completely remove default topbar if CSS isn't enough? No, we used CSS to repurpose it.
                    }
                }
            }
        ]
      });
    };
  </script>
</body>
</html>
`
