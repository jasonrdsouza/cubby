package main

import (
	"net/http"
	"strings"
)

// acceptsHTML returns true if the request's Accept header explicitly includes
// text/html. Wildcard accepts (like */*) do not count, so that curl and other
// programmatic clients receive raw content by default.
func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	for _, part := range strings.Split(accept, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if mediaType == "text/html" {
			return true
		}
	}
	return false
}

// hasTheme returns true if the given content type has a themed viewer.
func hasTheme(contentType string) bool {
	ct := strings.SplitN(contentType, ";", 2)[0]
	ct = strings.TrimSpace(ct)

	switch ct {
	case "text/markdown", "text/plain", "text/csv", "application/json":
		return true
	}
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	return false
}
