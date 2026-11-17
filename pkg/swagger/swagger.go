package swagger

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kvolt/kvolt/v2"
	"github.com/go-kvolt/kvolt/v2/context"
	"github.com/go-kvolt/kvolt/v2/internal/logx"
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
	if a == nil || a.engine == nil {
		return nil
	}
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
	// ScriptURL is the Scalar API reference script loaded by the docs page.
	// Default: DefaultScriptURL, a pinned release on the jsDelivr CDN checked against
	// DefaultScriptIntegrity. Set it to use another version or a copy you host yourself.
	ScriptURL string
	// ScriptIntegrity is the Subresource Integrity value for ScriptURL
	// (e.g. "sha384-..."). The browser refuses a script whose content does not match.
	// Ignored when ScriptURL is empty; leave empty for a script you serve yourself.
	ScriptIntegrity string
}

// The docs page loads a pinned Scalar release and checks its hash, so a change on the
// CDN cannot alter what runs on the page. To upgrade, change both values together.
const (
	// DefaultScriptURL is the script used when Config.ScriptURL is empty.
	DefaultScriptURL = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.78.1/dist/browser/standalone.js"
	// DefaultScriptIntegrity is the SHA-384 of the file at DefaultScriptURL.
	DefaultScriptIntegrity = "sha384-HVpqul+lojb2wwVK554SYr5rD2XqgJZqkYKGl4MyZarBSg+rHR9F0x1LVgzgj7JZ"
)

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
	if strings.TrimSpace(cfg.ScriptURL) == "" {
		cfg.ScriptURL = DefaultScriptURL
		cfg.ScriptIntegrity = DefaultScriptIntegrity
	}
	scriptAttrs := ""
	if integrity := strings.TrimSpace(cfg.ScriptIntegrity); integrity != "" {
		scriptAttrs = ` integrity="` + template.HTMLEscapeString(integrity) + `" crossorigin="anonymous"`
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
		logx.Printf("API Documentation enabled: %s/swagger/index.html\n", baseURL)
	}

	return func(c *context.Context) error {
		if c == nil || c.Writer == nil {
			return nil
		}
		if cfg.Disabled {
			return c.Status(404).String(404, "Not Found")
		}

		path := strings.TrimSuffix(c.Param("any"), "/")

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
		html := fmt.Sprintf(uiTemplate, template.HTMLEscapeString(cfg.Title), template.HTMLEscapeString(cfg.ScriptURL), scriptAttrs)
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
    <script src="%s"%s></script>
  </body>
</html>
`
