package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// writeBody writes the response body and logs a write failure (e.g. the client
// disconnected). The status line is already committed, so there is nothing to
// recover beyond logging.
//
// Callers pass either an embedded asset or a document rendered from a base URL
// that safeHost has already validated, so no client-controlled markup reaches
// the body. gosec's taint analysis cannot follow that validation.
func writeBody(w http.ResponseWriter, data []byte) {
	if _, err := w.Write(data); err != nil { // #nosec G705
		log.Printf("web: write error: %v", err)
	}
}

const baseURLPlaceholder = "https://finance.hermestech.uk"

// safeHost accepts only hostnames and optional ports built from characters that
// are inert in HTML, Markdown, XML and JSON. Validating instead of escaping lets
// one resolved base URL be reflected safely into all four document types; an
// HTML-escaped value would corrupt the non-HTML ones.
var safeHost = regexp.MustCompile(`^[A-Za-z0-9.\-]+(:[0-9]{1,5})?$`)

// resolveBaseURL determines the canonical origin for generated documents.
// BASE_URL is operator-supplied and trusted; otherwise the value is derived from
// the client-controlled Host header, which must pass safeHost before use.
func resolveBaseURL(r *http.Request, envBase string) string {
	if envBase != "" {
		return strings.TrimSuffix(envBase, "/")
	}
	if !safeHost.MatchString(r.Host) {
		return baseURLPlaceholder
	}
	scheme := "https"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "http" || proto == "https" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

// setBaseURLVary declares that the body depends on X-Forwarded-Proto, which no
// cache includes in its key. It is a no-op when BASE_URL is set, because the
// base URL is then a constant and adding Vary would fragment the cache for
// nothing. Host needs no Vary: it is already part of every cache key.
func setBaseURLVary(w http.ResponseWriter, envBase string) {
	if envBase == "" {
		w.Header().Set("Vary", "X-Forwarded-Proto")
	}
}

func contentETag(data []byte) string {
	return fmt.Sprintf(`"%x"`, sha256.Sum256(data))
}

// matchETag reports whether the If-None-Match header matches etag per RFC 7232.
// It parses the comma-separated list, honors the "*" wildcard, and compares
// using weak comparison (ignoring any W/ prefix on either side).
func matchETag(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "" {
		return false
	}
	if strings.TrimSpace(ifNoneMatch) == "*" {
		return true
	}
	candidate := strings.TrimPrefix(etag, "W/")
	for tag := range strings.SplitSeq(ifNoneMatch, ",") {
		if strings.TrimPrefix(strings.TrimSpace(tag), "W/") == candidate {
			return true
		}
	}
	return false
}

// serveContent applies the shared caching contract for documents: a content
// hash ETag, a Cache-Control policy, conditional 304 replies, and no body on
// HEAD. Any header the caller already set is preserved.
func serveContent(w http.ResponseWriter, r *http.Request, data []byte, contentType, cacheControl string) {
	etag := contentETag(data)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", etag)

	if matchETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		return
	}
	writeBody(w, data)
}

// generatedHandler renders a document from the request's base URL. The document
// is built from the endpoint catalog on every request, so it can never describe
// a route the server does not serve.
func generatedHandler(contentType, cacheControl string, render func(baseURL string) []byte) http.HandlerFunc {
	envBase := os.Getenv("BASE_URL")

	return func(w http.ResponseWriter, r *http.Request) {
		setBaseURLVary(w, envBase)
		serveContent(w, r, render(resolveBaseURL(r, envBase)), contentType, cacheControl)
	}
}

// webHandler serves the landing page, substituting the canonical base URL into
// the canonical, Open Graph and JSON-LD tags.
func webHandler(content fs.FS) http.HandlerFunc {
	raw, _ := fs.ReadFile(content, "index.html")
	envBase := os.Getenv("BASE_URL")

	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := resolveBaseURL(r, envBase)
		data := bytes.ReplaceAll(raw, []byte(baseURLPlaceholder), []byte(baseURL))

		setBaseURLVary(w, envBase)

		// Advertise both machine-readable twins for clients that never parse the
		// HTML head, such as AI crawlers doing a HEAD request first.
		w.Header().Set("Link", `</openapi.json>; rel="describedby"; type="application/json", `+
			`</index.html.md>; rel="alternate"; type="text/markdown"`)

		serveContent(w, r, data, "text/html; charset=utf-8", "public, max-age=300")
	}
}

// staticFileHandler serves an embedded asset that needs no base URL.
func staticFileHandler(content fs.FS, filename, contentType, cacheControl string) http.HandlerFunc {
	data, err := fs.ReadFile(content, filename)
	if err != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not found")
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		serveContent(w, r, data, contentType, cacheControl)
	}
}
