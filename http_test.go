package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/boltdb/bolt"
)

const (
	testUser     = "alice"
	testPassword = "secret"
	testAdmin    = "root"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cubby, err := NewCubbyServer(filepath.Join(t.TempDir(), "cubby.db"), 10)
	if err != nil {
		t.Fatalf("creating cubby server: %v", err)
	}
	t.Cleanup(cubby.Close)
	if err := cubby.AddUser(testUser, testPassword, false); err != nil {
		t.Fatalf("adding user: %v", err)
	}
	if err := cubby.AddUser(testAdmin, testPassword, true); err != nil {
		t.Fatalf("adding admin: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(cubby.Handler))
	t.Cleanup(ts.Close)
	return ts
}

type reqOpt func(*http.Request)

func withAuth(r *http.Request) { r.SetBasicAuth(testUser, testPassword) }

// withAdmin is used to observe missing keys, which only admins can read.
func withAdmin(r *http.Request) { r.SetBasicAuth(testAdmin, testPassword) }

func withHeader(name, value string) reqOpt {
	return func(r *http.Request) { r.Header.Set(name, value) }
}

func do(t *testing.T, ts *httptest.Server, method, key, body string, opts ...reqOpt) (*http.Response, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+"/"+key, reader)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, key, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return resp, string(b)
}

// put stores value at key as an authenticated user and returns the new ETag.
func put(t *testing.T, ts *httptest.Server, key, value string, opts ...reqOpt) string {
	t.Helper()
	opts = append([]reqOpt{withAuth, withHeader("Content-Type", "application/octet-stream")}, opts...)
	resp, _ := do(t, ts, http.MethodPost, key, value, opts...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: status %d", key, resp.StatusCode)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("POST %s: missing ETag", key)
	}
	return etag
}

func expectStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("%s %s: status %d, want %d", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want)
	}
}

func TestGetReturnsETagAndCacheHeaders(t *testing.T) {
	ts := newTestServer(t)
	data := "\x00\x01binary\xffdata"
	etag := put(t, ts, "photoframe", data)

	resp, body := do(t, ts, http.MethodGet, "photoframe", "")
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("ETag"); got != etag {
		t.Errorf("ETag = %q, want %q", got, etag)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if resp.Header.Get("Last-Modified") == "" {
		t.Error("missing Last-Modified")
	}
	if got := resp.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Expose-Headers"); !strings.Contains(got, "ETag") {
		t.Errorf("Access-Control-Expose-Headers = %q", got)
	}
	if body != data {
		t.Errorf("body = %q, want %q", body, data)
	}
}

func TestGetIfNoneMatchReturns304(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "photoframe", "image bytes")

	resp, body := do(t, ts, http.MethodGet, "photoframe", "", withHeader("If-None-Match", etag))
	expectStatus(t, resp, http.StatusNotModified)
	if body != "" {
		t.Errorf("304 body = %q, want empty", body)
	}
	if got := resp.Header.Get("ETag"); got != etag {
		t.Errorf("ETag = %q, want %q", got, etag)
	}
}

func TestPostChangesETag(t *testing.T) {
	ts := newTestServer(t)
	oldETag := put(t, ts, "photoframe", "v1")
	newETag := put(t, ts, "photoframe", "v2")
	if newETag == oldETag {
		t.Fatalf("ETag did not change across writes: %q", newETag)
	}

	resp, body := do(t, ts, http.MethodGet, "photoframe", "", withHeader("If-None-Match", oldETag))
	expectStatus(t, resp, http.StatusOK)
	if body != "v2" {
		t.Errorf("body = %q, want v2", body)
	}
	if got := resp.Header.Get("ETag"); got != newETag {
		t.Errorf("ETag = %q, want %q", got, newETag)
	}
}

func TestPrivateKeyNeverRevealsNotModified(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "secret", "private data", withHeader(CUBBY_READER_HEADER, "user"))

	resp, _ := do(t, ts, http.MethodGet, "secret", "", withHeader("If-None-Match", etag))
	expectStatus(t, resp, http.StatusUnauthorized)
	if resp.Header.Get("ETag") != "" {
		t.Error("unauthorized response leaked ETag")
	}

	resp, body := do(t, ts, http.MethodGet, "secret", "", withAuth)
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control = %q, want private, no-cache", got)
	}
	if body != "private data" {
		t.Errorf("body = %q", body)
	}
}

func TestThemedViewHasNoETagAndVariesOnAccept(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "pic", "fakepng", withHeader("Content-Type", "image/png"))

	resp, body := do(t, ts, http.MethodGet, "pic", "", withHeader("Accept", "text/html"))
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("themed Content-Type = %q", got)
	}
	if resp.Header.Get("ETag") != "" {
		t.Error("themed view must not carry the object ETag")
	}
	if got := resp.Header.Get("Vary"); got != "Accept" {
		t.Errorf("themed Vary = %q, want Accept", got)
	}
	if body == "fakepng" {
		t.Error("themed view returned raw bytes")
	}

	resp, body = do(t, ts, http.MethodGet, "pic?raw", "", withHeader("Accept", "text/html"))
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("ETag"); got != etag {
		t.Errorf("raw ETag = %q, want %q", got, etag)
	}
	if body != "fakepng" {
		t.Errorf("raw body = %q", body)
	}

	resp, _ = do(t, ts, http.MethodGet, "pic", "")
	expectStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("Vary"); got != "Accept" {
		t.Errorf("raw Vary = %q, want Accept", got)
	}
}

func TestHead(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "photoframe", "image bytes")
	put(t, ts, "secret", "private", withHeader(CUBBY_READER_HEADER, "user"))

	getResp, _ := do(t, ts, http.MethodGet, "photoframe", "")
	headResp, body := do(t, ts, http.MethodHead, "photoframe", "")
	expectStatus(t, headResp, http.StatusOK)
	if body != "" {
		t.Errorf("HEAD body = %q, want empty", body)
	}
	for _, h := range []string{"ETag", "Last-Modified", "Cache-Control", "Content-Type", "Content-Length"} {
		if got, want := headResp.Header.Get(h), getResp.Header.Get(h); got != want || got == "" {
			t.Errorf("HEAD %s = %q, GET %s = %q", h, got, h, want)
		}
	}

	resp, _ := do(t, ts, http.MethodHead, "photoframe", "", withHeader("If-None-Match", etag))
	expectStatus(t, resp, http.StatusNotModified)

	resp, _ = do(t, ts, http.MethodHead, "secret", "")
	expectStatus(t, resp, http.StatusUnauthorized)
}

func TestUnsupportedMethod(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := do(t, ts, http.MethodPut, "photoframe", "x", withAuth)
	expectStatus(t, resp, http.StatusMethodNotAllowed)
	allow := resp.Header.Get("Allow")
	for _, m := range []string{"GET", "HEAD", "POST", "DELETE"} {
		if !strings.Contains(allow, m) {
			t.Errorf("Allow = %q, missing %s", allow, m)
		}
	}
}

func TestPostIfMatch(t *testing.T) {
	ts := newTestServer(t)
	stale := put(t, ts, "k", "v1")
	current := put(t, ts, "k", "v2", withHeader("If-Match", stale))

	resp, _ := do(t, ts, http.MethodPost, "k", "v3", withAuth, withHeader("If-Match", stale))
	expectStatus(t, resp, http.StatusPreconditionFailed)
	if got := resp.Header.Get("ETag"); got != current {
		t.Errorf("412 ETag = %q, want current %q", got, current)
	}

	resp, body := do(t, ts, http.MethodGet, "k", "")
	if body != "v2" || resp.Header.Get("ETag") != current {
		t.Errorf("stored value changed after 412: body %q, ETag %q", body, resp.Header.Get("ETag"))
	}

	// a weak tag never matches under strong comparison
	resp, _ = do(t, ts, http.MethodPost, "k", "v3", withAuth, withHeader("If-Match", "W/"+current))
	expectStatus(t, resp, http.StatusPreconditionFailed)

	// any tag in a list may match
	put(t, ts, "k", "v3", withHeader("If-Match", `"1", `+current))
}

func TestPostIfNoneMatchStar(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, "existing", "v1")

	resp, _ := do(t, ts, http.MethodPost, "existing", "v2", withAuth, withHeader("If-None-Match", "*"))
	expectStatus(t, resp, http.StatusPreconditionFailed)
	if resp.Header.Get("ETag") == "" {
		t.Error("412 missing current ETag")
	}

	put(t, ts, "fresh", "v1", withHeader("If-None-Match", "*"))
}

func TestPostIfMatchStar(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := do(t, ts, http.MethodPost, "missing", "v1", withAuth, withHeader("If-Match", "*"))
	expectStatus(t, resp, http.StatusPreconditionFailed)
	if resp.Header.Get("ETag") != "" {
		t.Error("412 for missing key should not carry an ETag")
	}
	resp, _ = do(t, ts, http.MethodGet, "missing", "", withAdmin)
	expectStatus(t, resp, http.StatusNotFound)

	put(t, ts, "present", "v1")
	put(t, ts, "present", "v2", withHeader("If-Match", "*"))
}

func TestDeleteIfMatch(t *testing.T) {
	ts := newTestServer(t)
	stale := put(t, ts, "k", "v1")
	current := put(t, ts, "k", "v2")

	resp, _ := do(t, ts, http.MethodDelete, "k", "", withAuth, withHeader("If-Match", stale))
	expectStatus(t, resp, http.StatusPreconditionFailed)
	if got := resp.Header.Get("ETag"); got != current {
		t.Errorf("412 ETag = %q, want %q", got, current)
	}
	resp, body := do(t, ts, http.MethodGet, "k", "")
	expectStatus(t, resp, http.StatusOK)
	if body != "v2" {
		t.Errorf("body = %q, want v2", body)
	}

	resp, _ = do(t, ts, http.MethodDelete, "k", "", withAuth, withHeader("If-Match", current))
	expectStatus(t, resp, http.StatusOK)
	resp, _ = do(t, ts, http.MethodGet, "k", "", withAdmin)
	expectStatus(t, resp, http.StatusNotFound)
}

func TestUnauthenticatedConditionalWriteIs401(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "k", "v1")

	resp, _ := do(t, ts, http.MethodPost, "k", "v2", withHeader("If-Match", `"stale"`))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodPost, "k", "v2", withHeader("If-Match", etag))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodDelete, "k", "", withHeader("If-Match", `"stale"`))
	expectStatus(t, resp, http.StatusUnauthorized)

	// authenticated but not in the writer group: still 401, never 412
	put(t, ts, "admin-only", "v1", withHeader(CUBBY_WRITER_HEADER, "admin"))
	resp, _ = do(t, ts, http.MethodPost, "admin-only", "v2", withAuth, withHeader("If-Match", `"stale"`))
	expectStatus(t, resp, http.StatusUnauthorized)
}

func TestConcurrentConditionalWrites(t *testing.T) {
	ts := newTestServer(t)
	etag := put(t, ts, "k", "v0")

	const writers = 8
	statuses := make([]int, writers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/k", strings.NewReader("writer"))
			withAuth(req)
			req.Header.Set("If-Match", etag)
			<-start
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("writer %d: %v", i, err)
				return
			}
			resp.Body.Close()
			statuses[i] = resp.StatusCode
		}(i)
	}
	close(start)
	wg.Wait()

	var ok, failed int
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			ok++
		case http.StatusPreconditionFailed:
			failed++
		}
	}
	if ok != 1 || failed != writers-1 {
		t.Errorf("statuses = %v, want exactly one 200 and %d 412s", statuses, writers-1)
	}
}

func TestLegacyKeyWithoutUpdatedAt(t *testing.T) {
	cubby, err := NewCubbyServer(filepath.Join(t.TempDir(), "cubby.db"), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer cubby.Close()
	cubby.AddUser(testUser, testPassword, false)
	// legacy object: metadata predates UpdatedAt tracking
	err = cubby.db.Update(func(tx *bolt.Tx) error {
		if err := cubby.Put("legacy", []byte("old"), tx); err != nil {
			return err
		}
		return cubby.PutMetadata("legacy", &CubbyMetadata{ContentType: "text/x-legacy", Readers: PublicGroup, Writers: UserGroup}, tx)
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(cubby.Handler))
	defer ts.Close()

	resp, body := do(t, ts, http.MethodGet, "legacy", "")
	expectStatus(t, resp, http.StatusOK)
	if resp.Header.Get("ETag") != "" || resp.Header.Get("Last-Modified") != "" {
		t.Errorf("legacy key got validators: ETag %q, Last-Modified %q", resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"))
	}
	if body != "old" {
		t.Errorf("body = %q", body)
	}

	resp, _ = do(t, ts, http.MethodPost, "legacy", "new", withAuth, withHeader("If-Match", `"0"`))
	expectStatus(t, resp, http.StatusPreconditionFailed)
	put(t, ts, "legacy", "new", withHeader("If-Match", "*"))
}

func TestPagesRenderWithoutVCSInfo(t *testing.T) {
	saved := BuiltGitCommit
	BuiltGitCommit = "" // as in go test or builds outside a git checkout
	t.Cleanup(func() { BuiltGitCommit = saved })

	ts := newTestServer(t)
	put(t, ts, "notes", "# hello", withHeader("Content-Type", "text/markdown"))

	resp, body := do(t, ts, http.MethodGet, "", "")
	expectStatus(t, resp, http.StatusOK)
	if !strings.Contains(body, "notes") {
		t.Error("index page missing key listing")
	}

	resp, body = do(t, ts, http.MethodGet, "notes", "", withHeader("Accept", "text/html"))
	expectStatus(t, resp, http.StatusOK)
	if !strings.Contains(body, "unknown") {
		t.Error("themed view missing fallback version")
	}
}
