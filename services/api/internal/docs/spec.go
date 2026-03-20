// Package docs embeds the OpenAPI specification and exposes it for serving.
package docs

import _ "embed"

// OpenAPISpec is the raw openapi.yaml file, embedded at compile time.
//
//go:embed openapi.yaml
var OpenAPISpec []byte
