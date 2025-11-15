package handler

import (
	"net/http"
	"os"
)

// DocsHandler serves OpenAPI documentation
type DocsHandler struct {
	openapiPath string
}

// NewDocsHandler creates a docs handler
func NewDocsHandler(openapiPath string) *DocsHandler {
	return &DocsHandler{openapiPath: openapiPath}
}

// ServeOpenAPI serves the openapi.yml file
func (h *DocsHandler) ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(h.openapiPath)
	if err != nil {
		http.Error(w, "OpenAPI spec not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ServeSwaggerUI serves embedded Swagger UI HTML
func (h *DocsHandler) ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>PR Service API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5.10.3/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/openapi.yml",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
