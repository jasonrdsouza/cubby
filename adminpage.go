package main

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/boltdb/bolt"
)

// ADMIN_PATH is reserved for the admin page: it and everything under it can't
// be used as a storage key.
const ADMIN_PATH = "/_admin"

func isAdminPath(path string) bool {
	return path == ADMIN_PATH || strings.HasPrefix(path, ADMIN_PATH+"/")
}

func (c *CubbyServer) newAdminMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ADMIN_PATH, func(w http.ResponseWriter, r *http.Request) {
		c.renderAdminPage(w, http.StatusOK, adminNotice{})
	})
	mux.HandleFunc("POST "+ADMIN_PATH+"/keys", c.adminAddKey)
	mux.HandleFunc("POST "+ADMIN_PATH+"/keys/regenerate", c.adminRegenerateKey)
	mux.HandleFunc("POST "+ADMIN_PATH+"/keys/delete", c.adminDeleteKey)
	mux.HandleFunc("POST "+ADMIN_PATH+"/blobs/access", c.adminSetAccess)
	mux.HandleFunc("POST "+ADMIN_PATH+"/blobs/delete", c.adminDeleteBlob)
	return mux
}

// serveAdmin handles every request under ADMIN_PATH. Only admins get past the
// Basic Auth challenge; cross-site form posts are stopped by HTTPHandler.
func (c *CubbyServer) serveAdmin(w http.ResponseWriter, r *http.Request, user User) {
	if !user.IsAdmin() {
		log.Println("Unauthorized admin page attempt")
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// the page can show a freshly generated secret, so keep it out of caches
	w.Header().Set("Cache-Control", "no-store")
	c.adminMux.ServeHTTP(w, r)
}

// adminNotice is shown at the top of the admin page after an action.
type adminNotice struct {
	Error   string
	KeyName string // set with Secret when a reader key secret was generated
	Secret  string
}

func (c *CubbyServer) adminAddKey(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PostFormValue("name"))
	secret, err := c.AddReaderKey(name)
	c.renderNewSecret(w, name, secret, err)
}

func (c *CubbyServer) adminRegenerateKey(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	secret, err := c.RegenerateReaderKey(name)
	c.renderNewSecret(w, name, secret, err)
}

// renderNewSecret shows a just-generated secret on the page itself, rather
// than redirecting, so it never appears in a URL or the browser history.
func (c *CubbyServer) renderNewSecret(w http.ResponseWriter, name, secret string, err error) {
	if err != nil {
		c.renderAdminPage(w, http.StatusBadRequest, adminNotice{Error: err.Error()})
		return
	}
	c.renderAdminPage(w, http.StatusOK, adminNotice{KeyName: name, Secret: secret})
}

func (c *CubbyServer) adminDeleteKey(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	err := c.db.View(func(tx *bolt.Tx) error {
		if !c.isReaderKey(name, tx) {
			return fmt.Errorf("no reader key named %s", name)
		}
		return nil
	})
	if err == nil {
		err = c.RemoveUser(name)
	}
	c.redirectOrError(w, r, err)
}

func (c *CubbyServer) adminSetAccess(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.redirectOrError(w, r, err)
		return
	}
	public := r.PostForm.Get("access") == "public"
	c.redirectOrError(w, r, c.SetAccess(r.PostForm.Get("key"), public, r.PostForm["readers"]))
}

func (c *CubbyServer) adminDeleteBlob(w http.ResponseWriter, r *http.Request) {
	key := r.PostFormValue("key")
	err := c.db.Update(func(tx *bolt.Tx) error {
		if c.GetMetadata(key, tx).Empty() && tx.Bucket([]byte(c.dataBucket)).Get([]byte(key)) == nil {
			return fmt.Errorf("no blob named %s", key)
		}
		if err := c.Remove(key, tx); err != nil {
			return err
		}
		return c.RemoveMetadata(key, tx)
	})
	c.redirectOrError(w, r, err)
}

// redirectOrError finishes a form post: back to the admin page on success
// (post/redirect/get), or the page with the error shown.
func (c *CubbyServer) redirectOrError(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		c.renderAdminPage(w, http.StatusBadRequest, adminNotice{Error: err.Error()})
		return
	}
	http.Redirect(w, r, ADMIN_PATH, http.StatusSeeOther)
}

// SetAccess changes who can read an existing blob, leaving its content (and
// so its ETag) untouched.
func (c *CubbyServer) SetAccess(key string, public bool, readerKeys []string) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte(c.dataBucket)).Get([]byte(key)) == nil {
			return fmt.Errorf("no blob named %s", key)
		}
		for _, name := range readerKeys {
			if !c.isReaderKey(name, tx) {
				return fmt.Errorf("unknown reader key: %s", name)
			}
		}
		metadata := c.GetMetadata(key, tx)
		metadata.SetAccess(public, slices.Compact(slices.Sorted(slices.Values(readerKeys))))
		return c.PutMetadata(key, metadata, tx)
	})
}

type adminKeyRow struct {
	Name  string
	Blobs int // how many blobs list this key
}

type adminBlobRow struct {
	Key         string
	ContentType string
	Size        int
	UpdatedAt   string
	Public      bool
	ReaderKeys  []string
}

func (c *CubbyServer) renderAdminPage(w http.ResponseWriter, status int, notice adminNotice) {
	data := struct {
		adminNotice
		Admins       []string
		Keys         []adminKeyRow
		KeyNames     []string
		Blobs        []adminBlobRow
		Version      string
		ShortVersion string
	}{adminNotice: notice, Version: c.Version(), ShortVersion: c.ShortVersion()}

	keyIndex := map[string]int{}
	for _, user := range c.ListUsers() {
		if user.Admin {
			data.Admins = append(data.Admins, user.Name())
			continue
		}
		keyIndex[user.Name()] = len(data.Keys)
		data.Keys = append(data.Keys, adminKeyRow{Name: user.Name()})
		data.KeyNames = append(data.KeyNames, user.Name())
	}

	c.db.View(func(tx *bolt.Tx) error {
		for _, key := range c.List(tx) {
			metadata := c.GetMetadata(key, tx)
			row := adminBlobRow{
				Key:         key,
				ContentType: metadata.ContentType,
				Size:        len(tx.Bucket([]byte(c.dataBucket)).Get([]byte(key))),
				Public:      metadata.Public,
				ReaderKeys:  metadata.ReaderKeys,
			}
			if !metadata.UpdatedAt.IsZero() {
				row.UpdatedAt = metadata.UpdatedAt.Format("2006-01-02 15:04")
			}
			for _, name := range metadata.ReaderKeys {
				if i, ok := keyIndex[name]; ok {
					data.Keys[i].Blobs++
				}
			}
			data.Blobs = append(data.Blobs, row)
		}
		return nil
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.adminTemplate.Execute(w, data); err != nil {
		log.Printf("Error executing admin template: %v", err)
	}
}
