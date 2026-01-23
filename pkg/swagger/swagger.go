package swagger

// Config for Swagger UI
type Config struct {
	Title       string
	Description string
	Version     string
}

// GenerateSpec would parse the codebase (AST) to generate openapi.json
// For v1, we assume the user provides a spec file or we implement a basic reflector later.
func GenerateSpec(cfg Config) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":   cfg.Title,
			"version": cfg.Version,
		},
		"paths": map[string]interface{}{},
	}
}

// Handler returns a handler that serves the Swagger UI (HTML).
// Real implementation would serve the 'swagger-ui-dist' static files.
// Here we return a simple HTML page pointing to the spec.
func Handler(specURL string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <title>KVolt API Docs</title>
</head>
<body>
    <h1>Swagger UI Placeholder</h1>
    <p>Spec URL: ` + specURL + `</p>
</body>
</html>`
}
