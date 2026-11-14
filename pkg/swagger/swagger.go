package swagger

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
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
			Method:    r.Method,
			Path:      r.Path,
			Summary:   r.Summary,
			Tags:      r.Tags,
			Request:   r.Request,
			Responses: r.Responses,
			Security:  r.Security,
		})
	}
	return routes
}

// RouteProvider interface allows decoupling from direct kvolt.Engine dependency if needed.
type RouteProvider interface {
	Routes() []RouteInfo
}

// Config configures the Swagger UI.
type Config struct {
	// SpecJSON is the raw OpenAPI/Swagger JSON string.
	// If empty, and RoutesProvider is set, a spec is generated from routes.
	SpecJSON string
	// Title is the page title. Default: "KVolt API Docs"
	Title string
	// RoutesProvider supplies routes for auto-discovery.
	RoutesProvider interface {
		Routes() []RouteInfo
	}
	// Disabled, if true, disables the documentation endpoint (returns 404).
	Disabled bool
	// Host is the address where the server is running (e.g. "localhost:8080").
	Host string
}

// RouteInfo mirrors kvolt.RouteInfo for OpenAPI generation.
type RouteInfo struct {
	Method    string
	Path      string
	Summary   string
	Tags      []string
	Request   interface{}
	Responses map[int]interface{}
	Security  []string
}

// Handler returns a KVolt handler that serves the Swagger UI and the spec.
// Mount this at a path like "/swagger/*any".
func Handler(cfg Config) func(c *context.Context) error {
	if cfg.Title == "" {
		cfg.Title = "KVolt API Docs"
	}

	var (
		cachedSpec []byte
		specOnce   sync.Once
	)

	if !cfg.Disabled {
		baseURL := cfg.Host
		if baseURL == "" {
			baseURL = "localhost"
		}
		if !strings.HasPrefix(baseURL, "http") {
			baseURL = "http://" + baseURL
		}
		fmt.Printf("API Documentation enabled: %s/swagger/index.html\n", baseURL)
	}

	return func(c *context.Context) error {
		if c == nil || c.Writer == nil {
			return nil
		}
		if cfg.Disabled {
			return c.Status(404).String(404, "Not Found")
		}

		path := c.Param("any")

		if strings.HasSuffix(path, "doc.json") {
			c.SetHeaderWritten()
			c.Writer.Header().Set("Content-Type", "application/json")
			c.Writer.WriteHeader(200)

			if len(cfg.SpecJSON) > 0 {
				_, err := c.Writer.Write([]byte(cfg.SpecJSON))
				return err
			}

			if cfg.RoutesProvider != nil {
				specOnce.Do(func() {
					routes := cfg.RoutesProvider.Routes()
					cachedSpec = generateOpenAPI(routes, cfg.Title)
				})
			}

			if len(cachedSpec) > 0 {
				_, err := c.Writer.Write(cachedSpec)
				return err
			}

			_, err := c.Writer.Write([]byte("{}"))
			return err
		}

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

		var parameters []map[string]interface{}
		segments := strings.Split(r.Path, "/")
		for i, seg := range segments {
			if strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") {
				paramName := seg[1:]
				segments[i] = "{" + paramName + "}"
				parameters = append(parameters, map[string]interface{}{
					"name":     paramName,
					"in":       "path",
					"required": true,
					"schema":   map[string]string{"type": "string"},
				})
			}
		}
		openAPIPath := strings.Join(segments, "/")

		if paths[openAPIPath] == nil {
			paths[openAPIPath] = make(map[string]interface{})
		}

		method := strings.ToLower(r.Method)
		summary := r.Summary
		if summary == "" {
			summary = fmt.Sprintf("%s %s", r.Method, r.Path)
		}

		responses := map[string]interface{}{}
		if len(r.Responses) == 0 {
			responses["200"] = map[string]string{"description": "OK"}
		} else {
			for code, sample := range r.Responses {
				responses[strconv.Itoa(code)] = map[string]interface{}{
					"description": httpStatusText(code),
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": SchemaFromSample(sample),
						},
					},
				}
			}
		}

		operation := map[string]interface{}{
			"summary":   summary,
			"responses": responses,
		}
		if len(r.Tags) > 0 {
			operation["tags"] = r.Tags
		}
		if len(parameters) > 0 {
			operation["parameters"] = parameters
		}
		if r.Request != nil {
			operation["requestBody"] = map[string]interface{}{
				"required": true,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": SchemaFromSample(r.Request),
					},
				},
			}
		}
		if len(r.Security) > 0 {
			var sec []map[string][]string
			for _, s := range r.Security {
				sec = append(sec, map[string][]string{s: {}})
			}
			operation["security"] = sec
		}

		paths[openAPIPath][method] = operation
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

func httpStatusText(code int) string {
	if t := statusText(code); t != "" {
		return t
	}
	return strconv.Itoa(code)
}

func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return ""
	}
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
