package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
)

func (c *CubbyServer) Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path == "/" {
		// index page shows a list of occupied cubbies (ie. active keys)
		tmplData := struct {
			Keys         []string
			Version      string
			ShortVersion string
		}{
			Keys:         c.ListAtomic(),
			Version:      c.Version(),
			ShortVersion: c.Version()[:7],
		}

		err := c.indexTemplate.Execute(w, tmplData)
		if err != nil {
			http.Error(w, "Unable to generate template", http.StatusInternalServerError)
		}
		return
	}

	key := r.URL.Path[1:]
	username := r.Header.Get(USERNAME_HEADER)
	token := r.Header.Get(TOKEN_HEADER)
	user := c.FetchUser(username, token)

	if r.Method == http.MethodGet {
		c.db.View(func(tx *bolt.Tx) error {
			metadata := c.GetMetadata(key, tx)

			// auth check: reader allowlist
			if !user.InGroup(metadata.Readers) {
				http.Error(w, "Unauthorized Viewer", http.StatusUnauthorized)
				return nil
			}

			data := c.Get(key, tx)

			if len(data) == 0 && metadata.Empty() {
				http.NotFound(w, r)
			} else {
				w.Header().Set("Content-Type", metadata.ContentType)
				w.Header().Set("Last-Modified", metadata.UpdatedAt.Format(time.RFC1123))
				w.Write(data)
			}
			return nil
		})
	} else if r.Method == http.MethodPost {
		// auth check: disallow public writes
		if _, ok := user.(*AnonymousUser); ok {
			http.Error(w, "Unauthorized Writer", http.StatusUnauthorized)
			return
		}

		var b bytes.Buffer
		r.Body = http.MaxBytesReader(w, r.Body, c.maxObjectSize)
		_, err := b.ReadFrom(r.Body)
		if err != nil {
			http.Error(w, "Could not read data", http.StatusInternalServerError)
			return
		}

		err = c.db.Update(func(tx *bolt.Tx) error {
			metadata := c.GetMetadata(key, tx)

			// auth check: writer allowlist
			if !metadata.Empty() && !user.InGroup(metadata.Writers) {
				http.Error(w, "Unauthorized Overwrite", http.StatusUnauthorized)
				return nil
			}

			err := c.Put(key, b.Bytes(), tx)
			if err != nil {
				return err
			}

			readerGroup := StringToGroup(r.Header.Get("readerGroup"))
			if readerGroup == UnknownGroup {
				// allow public reads by default
				readerGroup = PublicGroup
			}
			writerGroup := StringToGroup(r.Header.Get("writerGroup"))
			if writerGroup == UnknownGroup {
				// allow authenticated user writes by default
				writerGroup = UserGroup
			}

			metadata = &CubbyMetadata{
				ContentType: r.Header.Get("Content-Type"),
				UpdatedAt:   time.Now(),
				Readers:     readerGroup,
				Writers:     writerGroup,
			}
			return c.PutMetadata(key, metadata, tx)
		})
		if err != nil {
			http.Error(w, "Could not persist data", http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodDelete {
		// auth check: disallow public deletes
		if _, ok := user.(*AnonymousUser); ok {
			http.Error(w, "Unauthorized Writer", http.StatusUnauthorized)
			return
		}

		err := c.db.Update(func(tx *bolt.Tx) error {
			metadata := c.GetMetadata(key, tx)

			// auth check: writer allowlist
			if !user.InGroup(metadata.Writers) {
				http.Error(w, "Unauthorized Writer", http.StatusUnauthorized)
				return nil
			}

			err := c.Remove(key, tx)
			if err != nil {
				return err
			}
			return c.RemoveMetadata(key, tx)
		})

		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		fmt.Fprintf(w, "Invalid action for key: %s", key)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
