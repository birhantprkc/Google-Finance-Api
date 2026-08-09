package web

import "embed"

// openapi.json, llms.txt, index.html.md, sitemap.xml and robots.txt are not
// embedded: they are rendered from the endpoint catalog in internal/api.
//
//go:embed index.html VERSION favicon.svg
var Content embed.FS
