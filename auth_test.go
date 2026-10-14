package main

import (
	"bytes"
	"encoding/gob"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/boltdb/bolt"
)

func TestReaderKeyAccess(t *testing.T) {
	ts, cubby := newTestCubby(t)
	addTestReaderKey(t, cubby, "bob")
	put(t, ts, "pub", "public data")
	put(t, ts, "family", "alice data", withHeader(CUBBY_READER_HEADER, testReader))
	put(t, ts, "work", "bob data", withHeader(CUBBY_READER_HEADER, "bob"))
	put(t, ts, "shared", "both", withHeader(CUBBY_READER_HEADER, "bob, alice"))
	put(t, ts, "mine", "admin data", withHeader(CUBBY_READER_HEADER, "private"))

	cases := []struct {
		who  string
		auth []reqOpt
		want map[string]int
	}{
		{"anonymous", nil, map[string]int{"pub": 200, "family": 401, "work": 401, "shared": 401, "mine": 401}},
		{"alice", []reqOpt{withReader}, map[string]int{"pub": 200, "family": 200, "work": 401, "shared": 200, "mine": 401}},
		{"bob", []reqOpt{withBasicAuth("bob", testPassword)}, map[string]int{"pub": 200, "family": 401, "work": 200, "shared": 200, "mine": 401}},
		{"wrong password", []reqOpt{withBasicAuth(testReader, "nope")}, map[string]int{"pub": 200, "family": 401, "shared": 401}},
		{"admin", []reqOpt{withAdmin}, map[string]int{"pub": 200, "family": 200, "work": 200, "shared": 200, "mine": 200}},
	}
	for _, tc := range cases {
		for key, want := range tc.want {
			resp, _ := do(t, ts, http.MethodGet, key, "", tc.auth...)
			if resp.StatusCode != want {
				t.Errorf("%s GET %s: status %d, want %d", tc.who, key, resp.StatusCode, want)
			}
		}
	}
}

func TestReaderKeyCannotWrite(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, "family", "v1", withHeader(CUBBY_READER_HEADER, testReader))

	for _, key := range []string{"family", "new-key"} {
		resp, _ := do(t, ts, http.MethodPost, key, "evil", withReader)
		expectStatus(t, resp, http.StatusUnauthorized)
		resp, _ = do(t, ts, http.MethodDelete, key, "", withReader)
		expectStatus(t, resp, http.StatusUnauthorized)
	}

	resp, body := do(t, ts, http.MethodGet, "family", "", withReader)
	expectStatus(t, resp, http.StatusOK)
	if body != "v1" {
		t.Errorf("body = %q, want v1", body)
	}
	resp, _ = do(t, ts, http.MethodGet, "new-key", "", withAdmin)
	expectStatus(t, resp, http.StatusNotFound)
}

func TestReaderHeaderValidation(t *testing.T) {
	ts := newTestServer(t)
	for _, header := range []string{"nobody", testAdmin, "alice,nobody", "public,alice", "PUBLIC,alice", "user", "bad name", "alice,"} {
		resp, _ := do(t, ts, http.MethodPost, "k", "v", withAdmin, withHeader(CUBBY_READER_HEADER, header))
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s %q: status %d, want 400", CUBBY_READER_HEADER, header, resp.StatusCode)
		}
	}
	resp, _ := do(t, ts, http.MethodGet, "k", "", withAdmin)
	expectStatus(t, resp, http.StatusNotFound)
}

func TestAccessPersistsAcrossWrites(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, "k", "v1", withHeader(CUBBY_READER_HEADER, testReader))
	put(t, ts, "k", "v2") // no header: keeps the existing access

	resp, _ := do(t, ts, http.MethodGet, "k", "")
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, body := do(t, ts, http.MethodGet, "k", "", withReader)
	expectStatus(t, resp, http.StatusOK)
	if body != "v2" {
		t.Errorf("body = %q, want v2", body)
	}

	put(t, ts, "k", "v3", withHeader(CUBBY_READER_HEADER, "public"))
	resp, _ = do(t, ts, http.MethodGet, "k", "")
	expectStatus(t, resp, http.StatusOK)

	// public and private are case-insensitive
	put(t, ts, "k", "v4", withHeader(CUBBY_READER_HEADER, "PRIVATE"))
	resp, _ = do(t, ts, http.MethodGet, "k", "")
	expectStatus(t, resp, http.StatusUnauthorized)
	put(t, ts, "k", "v5", withHeader(CUBBY_READER_HEADER, " Public "))
	resp, _ = do(t, ts, http.MethodGet, "k", "")
	expectStatus(t, resp, http.StatusOK)
}

// X-Cubby-Reader states the complete access, so a shorter list revokes the
// keys it leaves out.
func TestReaderHeaderReplacesKeyList(t *testing.T) {
	ts, cubby := newTestCubby(t)
	addTestReaderKey(t, cubby, "work")
	put(t, ts, "k", "v1", withHeader(CUBBY_READER_HEADER, "work,"+testReader))
	put(t, ts, "k", "v2", withHeader(CUBBY_READER_HEADER, "work"))

	resp, _ := do(t, ts, http.MethodGet, "k", "", withReader)
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, body := do(t, ts, http.MethodGet, "k", "", withBasicAuth("work", testPassword))
	expectStatus(t, resp, http.StatusOK)
	if body != "v2" {
		t.Errorf("body = %q, want v2", body)
	}
}

func TestRemovingReaderKeyRevokesItsBlobs(t *testing.T) {
	ts, cubby := newTestCubby(t)
	put(t, ts, "family", "data", withHeader(CUBBY_READER_HEADER, testReader))

	if err := cubby.RemoveUser(testReader); err != nil {
		t.Fatal(err)
	}
	resp, _ := do(t, ts, http.MethodGet, "family", "", withReader)
	expectStatus(t, resp, http.StatusUnauthorized)

	// a new key reusing the name must not inherit the old key's access
	addTestReaderKey(t, cubby, testReader)
	resp, _ = do(t, ts, http.MethodGet, "family", "", withReader)
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodGet, "family", "", withAdmin)
	expectStatus(t, resp, http.StatusOK)
}

func TestAddReaderKey(t *testing.T) {
	ts, cubby := newTestCubby(t)
	secret, err := cubby.AddReaderKey("guest")
	if err != nil {
		t.Fatal(err)
	}
	put(t, ts, "k", "v", withHeader(CUBBY_READER_HEADER, "guest"))
	resp, _ := do(t, ts, http.MethodGet, "k", "", withBasicAuth("guest", secret))
	expectStatus(t, resp, http.StatusOK)

	if _, err := cubby.AddReaderKey("guest"); err == nil {
		t.Error("adding an existing reader key succeeded")
	}
	if _, err := cubby.RegenerateReaderKey(testAdmin); err == nil {
		t.Error("regenerating an admin as a reader key succeeded")
	}
	if _, err := cubby.AddReaderKey(testAdmin); err == nil {
		t.Error("adding a reader key with an admin's name succeeded")
	}
	for _, reserved := range []string{"public", "Private"} {
		if _, err := cubby.AddReaderKey(reserved); err == nil {
			t.Errorf("adding reader key %q succeeded", reserved)
		}
	}
	if _, err := cubby.RegenerateReaderKey("nobody"); err == nil {
		t.Error("regenerating a missing reader key succeeded")
	}
	if err := cubby.AddAdmin("guest", "pw"); err == nil {
		t.Error("promoting a reader key to admin succeeded")
	}

	newSecret, err := cubby.RegenerateReaderKey("guest")
	if err != nil {
		t.Fatal(err)
	}
	resp, _ = do(t, ts, http.MethodGet, "k", "", withBasicAuth("guest", secret))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodGet, "k", "", withBasicAuth("guest", newSecret))
	expectStatus(t, resp, http.StatusOK)
}

func TestAdminPasswordLength(t *testing.T) {
	_, cubby := newTestCubby(t)
	if err := cubby.AddAdmin("long", strings.Repeat("x", 73)); err == nil || !strings.Contains(err.Error(), "72 bytes") {
		t.Errorf("73-byte password: err = %v, want a 72-byte limit error", err)
	}
	if err := cubby.AddAdmin("long", strings.Repeat("x", 72)); err != nil {
		t.Errorf("72-byte password: %v", err)
	}
}

// Key names are public, like labels on cubbies; only contents are protected.
func TestIndexListsAllKeys(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, "pub-blob", "v")
	put(t, ts, "alice-blob", "v", withHeader(CUBBY_READER_HEADER, testReader))
	put(t, ts, "admin-blob", "v", withHeader(CUBBY_READER_HEADER, "private"))

	resp, body := do(t, ts, http.MethodGet, "", "")
	expectStatus(t, resp, http.StatusOK)
	for _, key := range []string{"pub-blob", "alice-blob", "admin-blob"} {
		if !strings.Contains(body, key) {
			t.Errorf("anonymous index missing %s", key)
		}
	}
	resp, _ = do(t, ts, http.MethodGet, "alice-blob", "")
	expectStatus(t, resp, http.StatusUnauthorized)
}

// Records written before the reader-key model must decode with the least
// privilege, so an un-migrated database leaks nothing.
func TestPreMigrationRecordsFailClosed(t *testing.T) {
	ts, cubby := newTestCubby(t)

	type oldMetadata struct {
		ContentType string
		UpdatedAt   time.Time
		Readers     int
		Writers     int
	}
	type oldUser struct {
		Username     string
		PasswordHash []byte
		Groups       []int
	}
	const oldAdmin, oldPublic, oldUserGroup = 1, 3, 2
	encode := func(v any) []byte {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(v); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	user, err := NewUser("x", testPassword, false)
	if err != nil {
		t.Fatal(err)
	}
	hash := user.PasswordHash
	err = cubby.db.Update(func(tx *bolt.Tx) error {
		tx.Bucket([]byte(cubby.dataBucket)).Put([]byte("was-public"), []byte("data"))
		tx.Bucket([]byte(cubby.metaBucket)).Put([]byte("was-public"),
			encode(oldMetadata{ContentType: "text/plain", UpdatedAt: time.Now(), Readers: oldPublic, Writers: oldUserGroup}))
		return tx.Bucket([]byte(cubby.usersBucket)).Put([]byte("oldadmin"),
			encode(oldUser{Username: "oldadmin", PasswordHash: hash, Groups: []int{oldPublic, oldUserGroup, oldAdmin}}))
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, _ := do(t, ts, http.MethodGet, "was-public", "")
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodGet, "was-public", "", withBasicAuth("oldadmin", testPassword))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, _ = do(t, ts, http.MethodPost, "was-public", "evil", withBasicAuth("oldadmin", testPassword))
	expectStatus(t, resp, http.StatusUnauthorized)
	resp, body := do(t, ts, http.MethodGet, "was-public", "", withAdmin)
	expectStatus(t, resp, http.StatusOK)
	if body != "data" {
		t.Errorf("body = %q", body)
	}
}

func TestCrossSiteWritesRejected(t *testing.T) {
	ts := newTestServer(t)
	put(t, ts, "k", "v1")

	crossSite := []reqOpt{withAdmin, withHeader("Sec-Fetch-Site", "cross-site"), withHeader("Origin", "https://evil.example")}
	resp, _ := do(t, ts, http.MethodPost, "k", "evil", crossSite...)
	expectStatus(t, resp, http.StatusForbidden)
	resp, _ = do(t, ts, http.MethodDelete, "k", "", crossSite...)
	expectStatus(t, resp, http.StatusForbidden)

	// cross-origin reads and CORS preflights still work
	resp, body := do(t, ts, http.MethodGet, "k", "", crossSite...)
	expectStatus(t, resp, http.StatusOK)
	if body != "v1" {
		t.Errorf("body = %q, want v1", body)
	}
	resp, _ = do(t, ts, http.MethodOptions, "k", "", crossSite...)
	expectStatus(t, resp, http.StatusNoContent)

	// same-origin pages and trusted origins may write
	put(t, ts, "k", "v2", withHeader("Sec-Fetch-Site", "same-origin"))
	put(t, ts, "k", "v3", withHeader("Sec-Fetch-Site", "cross-site"), withHeader("Origin", trustedOrigin))
}
