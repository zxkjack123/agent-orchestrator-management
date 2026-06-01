// Package webui exposes the compiled frontend bundle for embedding into the
// aom binary. Run `cd web && npm run build` to populate the dist/ directory
// before building the Go binary.
package webui

import "embed"

//go:embed all:dist
var FS embed.FS
