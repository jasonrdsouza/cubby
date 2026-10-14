package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var withForm = withHeader("Content-Type", "application/x-www-form-urlencoded")

// adminPost submits an admin page form as an admin. Successful actions redirect
// back to the page, which the client follows.
func adminPost(t *testing.T, ts *httptest.Server, path string, form url.Values, opts ...reqOpt) (*http.Response, string) {
	t.Helper()
	opts = append([]reqOpt{withAdmin, withForm}, opts...)
	return do(t, ts, http.MethodPost, "_admin/"+path, form.Encode(), opts...)
}

var secretPattern = regexp.MustCompile(`<code class="secret">([^<]+)</code>`)

func createKeyViaPage(t *testing.T, ts *httptest.Server, name string, path string) string {
	t.Helper()
	resp, body := adminPost(t, ts, path, url.Values{"name": {name}})
	expectStatus(t, resp, http.StatusOK)
	m := secretPattern.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("%s: no secret shown", path)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	return m[1]
}

func TestAdminPageRequiresAdmin(t *testing.T) {
	ts := newTestServer(t)
	for _, auth := range [][]reqOpt{nil, {withReader}, {withBasicAuth(testAdmin, "wrong")}} {
		resp, _ := do(t, ts, http.MethodGet, "_admin", "", auth...)
		expectStatus(t, resp, http.StatusUnauthorized)
		if resp.Header.Get("WWW-Authenticate") == "" {
			t.Error("401 without a Basic Auth challenge")
		}
		resp, _ = do(t, ts, http.MethodPost, "_admin/keys", "name=sneaky", append(auth, withForm)...)
		expectStatus(t, resp, http.StatusUnauthorized)
	}
	resp, body := do(t, ts, http.MethodGet, "_admin", "", withAdmin)
	expectStatus(t, resp, http.StatusOK)
	if !strings.Contains(body, testReader) {
		t.Error("admin page missing reader key")
	}
	if strings.Contains(body, "sneaky") {
		t.Error("unauthorized form post created a reader key")
	}
}

func TestAdminPathIsReserved(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := do(t, ts, http.MethodPost, "_admin", "data", withAdmin)
	if resp.StatusCode == http.StatusOK {
		t.Error("POST /_admin stored a blob")
	}
	resp, _ = do(t, ts, http.MethodPost, "_admin/notes", "data", withAdmin)
	if resp.StatusCode == http.StatusOK {
		t.Error("POST /_admin/notes stored a blob")
	}
	// keys merely starting with the prefix are ordinary
	put(t, ts, "_administrivia", "data")
}

func TestAdminPageManagesKeys(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, "family", "photos", withHeader(CUBBY_READER_HEADER, "private"))

	secret := createKeyViaPage(t, ts, "guest", "keys")
	resp, body := adminPost(t, ts, "blobs/access", url.Values{"key": {"family"}, "access": {"private"}, "readers": {"guest", testReader}})
	expectStatus(t, resp, http.StatusOK)
	if !strings.Contains(body, `value="guest" checked`) {
		t.Error("admin page doesn't show guest checked for family")
	}

	resp, body = do(t, ts, http.MethodGet, "family", "", withBasicAuth("guest", secret))
	expectStatus(t, resp, http.StatusOK)
	if body != "photos" {
		t.Errorf("body = %q", body)
	}

	// duplicate and invalid names are shown as errors
	resp, body = adminPost(t, ts, "keys", url.Values{"name": {"guest"}})
	expectStatus(t, resp, http.StatusBadRequest)
	if !strings.Contains(body, "already exists") {
		t.Error("duplicate key error not shown")
	}
	resp, _ = adminPost(t, ts, "keys", url.Values{"name": {"bad name"}})
	expectStatus(t, resp, http.StatusBadRequest)

	newSecret := createKeyViaPage(t, ts, "guest", "keys/regenerate")
	resp, _ = do(t, ts, http.MethodGet, "family", "", withBasicAuth("guest", secret))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodGet, "family", "", withBasicAuth("guest", newSecret))
	expectStatus(t, resp, http.StatusOK)

	// admins can't be deleted or regenerated from the page
	resp, _ = adminPost(t, ts, "keys/delete", url.Values{"name": {testAdmin}})
	expectStatus(t, resp, http.StatusBadRequest)
	resp, _ = adminPost(t, ts, "keys/regenerate", url.Values{"name": {testAdmin}})
	expectStatus(t, resp, http.StatusBadRequest)

	resp, _ = adminPost(t, ts, "keys/delete", url.Values{"name": {"guest"}})
	expectStatus(t, resp, http.StatusOK)
	resp, _ = do(t, ts, http.MethodGet, "family", "", withBasicAuth("guest", newSecret))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodGet, "family", "", withReader)
	expectStatus(t, resp, http.StatusOK)
}

func TestAdminPageManagesBlobs(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "notes", "text")

	resp, _ := adminPost(t, ts, "blobs/access", url.Values{"key": {"notes"}, "access": {"private"}})
	expectStatus(t, resp, http.StatusOK)
	resp, _ = do(t, ts, http.MethodGet, "notes", "")
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodGet, "notes", "", withAdmin)
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("ETag"); got != etag {
		t.Errorf("access change altered ETag: %q, want %q", got, etag)
	}

	resp, _ = adminPost(t, ts, "blobs/access", url.Values{"key": {"notes"}, "access": {"private"}, "readers": {"nobody"}})
	expectStatus(t, resp, http.StatusBadRequest)
	resp, _ = adminPost(t, ts, "blobs/access", url.Values{"key": {"missing"}, "access": {"public"}})
	expectStatus(t, resp, http.StatusBadRequest)

	resp, _ = adminPost(t, ts, "blobs/access", url.Values{"key": {"notes"}, "access": {"public"}, "readers": {testReader}})
	expectStatus(t, resp, http.StatusOK)
	resp, _ = do(t, ts, http.MethodGet, "notes", "")
	expectStatus(t, resp, http.StatusOK)

	resp, _ = adminPost(t, ts, "blobs/delete", url.Values{"key": {"notes"}})
	expectStatus(t, resp, http.StatusOK)
	resp, _ = do(t, ts, http.MethodGet, "notes", "", withAdmin)
	expectStatus(t, resp, http.StatusNotFound)
}

func TestAdminPageRejectsCrossSiteForms(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := adminPost(t, ts, "keys", url.Values{"name": {"planted"}},
		withHeader("Sec-Fetch-Site", "cross-site"), withHeader("Origin", "https://evil.example"))
	expectStatus(t, resp, http.StatusForbidden)

	_, body := do(t, ts, http.MethodGet, "_admin", "", withAdmin)
	if strings.Contains(body, "planted") {
		t.Error("cross-site form created a reader key")
	}
	createKeyViaPage(t, ts, "ok", "keys")
}

func TestAdminPageEscapesBlobKeys(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, `x"><script>alert(1)</script>`, "v")
	_, body := do(t, ts, http.MethodGet, "_admin", "", withAdmin)
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("blob key rendered unescaped")
	}
}
