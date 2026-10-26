// Package docs embeds the published OpenAPI contract and Swagger UI entrypoint.
package docs

import _ "embed"

// OpenAPI is the authoritative API schema.
//
//go:embed openapi.yaml
var OpenAPI []byte

// SwaggerHTML loads a versioned Swagger UI distribution.
const SwaggerHTML = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Warehouse API</title><link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.29.5/swagger-ui.css"></head><body><div id="swagger-ui"></div><script src="https://unpkg.com/swagger-ui-dist@5.29.5/swagger-ui-bundle.js"></script><script>SwaggerUIBundle({url:'/docs/openapi.yaml',dom_id:'#swagger-ui',persistAuthorization:false});</script></body></html>`
