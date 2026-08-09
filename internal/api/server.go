package api

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/kilimcininkoroglu/google-finance-api/internal/gfrpc"
)

func NewServer(ctx context.Context, client *gfrpc.Client, port string, webFS fs.FS) *http.Server {
	hub := newLiveHub(client)
	go hub.run(ctx)

	version := "unknown"
	if v, err := fs.ReadFile(webFS, "VERSION"); err == nil {
		version = strings.TrimSpace(string(v))
	}

	h := &handlers{client: client, hub: hub}
	endpoints := apiEndpoints(h, version)

	mux := http.NewServeMux()

	// Routes come from the same catalog that renders openapi.json, llms.txt and
	// index.html.md, so a documented endpoint is always a served endpoint.
	for _, e := range endpoints {
		mux.HandleFunc("GET "+e.Path, e.Handler)
	}

	mux.HandleFunc("GET /{$}", webHandler(webFS))
	mux.HandleFunc("GET /index.html.md", generatedHandler(
		"text/markdown; charset=utf-8", "public, max-age=3600",
		func(baseURL string) []byte { return renderIndexMarkdown(baseURL, endpoints) }))
	mux.HandleFunc("GET /llms.txt", generatedHandler(
		"text/plain; charset=utf-8", "public, max-age=3600",
		func(baseURL string) []byte { return renderLLMsTxt(baseURL, endpoints) }))
	mux.HandleFunc("GET /openapi.json", generatedHandler(
		"application/json; charset=utf-8", "public, max-age=3600",
		func(baseURL string) []byte { return renderOpenAPI(baseURL, version, endpoints) }))
	mux.HandleFunc("GET /sitemap.xml", generatedHandler(
		"application/xml; charset=utf-8", "public, max-age=3600", renderSitemap))
	mux.HandleFunc("GET /robots.txt", generatedHandler(
		"text/plain; charset=utf-8", "public, max-age=3600", renderRobots))
	mux.HandleFunc("GET /favicon.svg", staticFileHandler(webFS, "favicon.svg", "image/svg+xml", "public, max-age=86400"))

	var handler http.Handler = mux
	handler = loggingMiddleware(handler)
	handler = recoveryMiddleware(handler)
	handler = corsMiddleware(handler)

	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
