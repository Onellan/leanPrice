// Package web contains the server-rendered templates and local static assets.
package web

import "embed"

// Assets contains the application templates and locally served files.
//
//go:embed templates static
var Assets embed.FS
