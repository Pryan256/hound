package handlers

import (
	"net/http"

	"github.com/hound-fi/api/internal/docs"
)

// OpenAPISpec serves the raw openapi.yaml file.
func (h *Handler) OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write(docs.OpenAPISpec)
}

// Docs serves an interactive Redoc API reference page.
// The spec is loaded from /openapi.yaml on the same origin.
func (h *Handler) Docs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(docsHTML))
}

const docsHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Hound API Reference</title>
  <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🐕</text></svg>">
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <redoc spec-url="/openapi.yaml"
         expand-responses="200,201"
         theme='{"colors":{"primary":{"main":"#2D2800"}},"typography":{"fontFamily":"system-ui,-apple-system,sans-serif","headings":{"fontFamily":"system-ui,-apple-system,sans-serif"}}}'
  ></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc@latest/bundles/redoc.standalone.js"></script>
</body>
</html>`
