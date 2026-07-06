// Package swaggerui embeds the Swagger UI host page.
package swaggerui

import _ "embed"

// IndexHTML is the Swagger UI host page served at /swagger.
//
//go:embed index.html
var IndexHTML []byte
