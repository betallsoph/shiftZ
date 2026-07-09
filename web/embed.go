// Package web embeds the built dashboard frontend. The dashboard itself
// comes later; dist/ currently holds a placeholder page so the embed and the
// serving path in cmd/server are wired end to end.
//
// When the frontend lands, its build step outputs into web/dist and nothing
// here changes.
package web

import "embed"

// Dist is the built frontend, served by cmd/server at /.
//
//go:embed all:dist
var Dist embed.FS
