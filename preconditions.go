package main

import (
	"net/http"
	"strings"
)

// writePreconditionsPass evaluates the If-Match and If-None-Match request
// headers against the current state of a key (RFC 9110 §13.2.2), using strong
// comparison only. exists reports whether the key currently exists, and etag is
// its current ETag ("" if it has none). It returns false if the request must
// be rejected with 412 Precondition Failed.
func writePreconditionsPass(r *http.Request, exists bool, etag string) bool {
	if ifMatch := r.Header.Values("If-Match"); len(ifMatch) > 0 {
		if !etagListMatches(strings.Join(ifMatch, ","), exists, etag) {
			return false
		}
	}
	if ifNoneMatch := r.Header.Values("If-None-Match"); len(ifNoneMatch) > 0 {
		if etagListMatches(strings.Join(ifNoneMatch, ","), exists, etag) {
			return false
		}
	}
	return true
}

// etagListMatches reports whether an If-Match / If-None-Match field value
// matches the current representation. "*" matches any existing key; a listed
// entity tag matches only if it is strong and identical to the current ETag.
func etagListMatches(header string, exists bool, current string) bool {
	s := header
	for {
		s = strings.TrimLeft(s, " \t,")
		if s == "" {
			return false
		}
		if s[0] == '*' {
			if exists {
				return true
			}
			s = s[1:]
			continue
		}

		weak := false
		if strings.HasPrefix(s, "W/") {
			weak = true
			s = s[2:]
		}
		if s == "" || s[0] != '"' {
			// malformed entry: skip to the next list element
			if i := strings.IndexByte(s, ','); i >= 0 {
				s = s[i+1:]
				continue
			}
			return false
		}
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return false
		}
		tag := s[:end+2]
		s = s[end+2:]

		if !weak && current != "" && tag == current {
			return true
		}
	}
}
