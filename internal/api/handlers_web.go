package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

const baseURLPlaceholder = "https://finance.hermestech.uk"

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
	for _, tag := range strings.Split(ifNoneMatch, ",") {
		if strings.TrimPrefix(strings.TrimSpace(tag), "W/") == candidate {
			return true
		}
	}
	return false
}

func webHandler(content fs.FS) http.HandlerFunc {
	raw, _ := fs.ReadFile(content, "index.html")
	envBase := os.Getenv("BASE_URL")

	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := envBase
		if baseURL == "" {
			scheme := "https"
			if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
				scheme = proto
			}
			baseURL = scheme + "://" + r.Host
		}
		data := bytes.ReplaceAll(raw, []byte(baseURLPlaceholder), []byte(baseURL))
		etag := contentETag(data)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("ETag", etag)
		w.Header().Set("Link", `</openapi.json>; rel="describedby"; type="application/json"`)

		if matchETag(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.Method == http.MethodHead {
			return
		}
		w.Write(data)
	}
}

func staticFileHandler(content fs.FS, filename, contentType, cacheControl string) http.HandlerFunc {
	data, err := fs.ReadFile(content, filename)
	if err != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not found")
		}
	}
	etag := contentETag(data)

	return func(w http.ResponseWriter, r *http.Request) {
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
		w.Write(data)
	}
}

func openAPIHandler(content fs.FS) http.HandlerFunc {
	data, _ := fs.ReadFile(content, "openapi.json")
	etag := contentETag(data)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("ETag", etag)
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if matchETag(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.Method == http.MethodHead {
			return
		}
		w.Write(data)
	}
}
