package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/boltdb/bolt"
)

func (c *CubbyServer) Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" || r.URL.Path == "/" {
		log.Println("Serving index page")
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
	username, password, ok := r.BasicAuth()
	if !ok {
		// set empty username and password to fetch AnonymousUser
		username = ""
		password = ""
	}
	user := c.FetchUser(username, password)

	if r.Method == http.MethodGet {
		c.db.View(func(tx *bolt.Tx) error {
			metadata := c.GetMetadata(key, tx)
			log.Printf("Fetched metadata for key %s: %s", key, metadata)

			// auth check: reader allowlist
			if !user.InGroup(metadata.Readers) {
				log.Println("Unauthorized read attempt")
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
				http.Error(w, "Unauthorized Reader", http.StatusUnauthorized)
				return nil
			}

			data := c.Get(key, tx)

			if len(data) == 0 && metadata.Empty() {
				log.Printf("Key %s not found", key)
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
			log.Println("Unauthorized write attempt")
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized Writer", http.StatusUnauthorized)
			return
		}

		var b bytes.Buffer
		r.Body = http.MaxBytesReader(w, r.Body, c.maxObjectSize)
		_, err := b.ReadFrom(r.Body)
		if err != nil {
			log.Printf("Error reading uploaded data: %v", err)
			http.Error(w, "Could not read data", http.StatusInternalServerError)
			return
		}

		err = c.db.Update(func(tx *bolt.Tx) error {
			metadata := c.GetMetadata(key, tx)

			// auth check: writer allowlist
			if !metadata.Empty() && !user.InGroup(metadata.Writers) {
				log.Println("Unauthorized overwrite attempt")
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
				http.Error(w, "Unauthorized Overwrite", http.StatusUnauthorized)
				return nil
			}

			err := c.Put(key, b.Bytes(), tx)
			if err != nil {
				return err
			}

			metadata.UpdateReaders(StringToGroup(r.Header.Get(CUBBY_READER_HEADER)))
			metadata.UpdateWriters(StringToGroup(r.Header.Get(CUBBY_WRITER_HEADER)))
			metadata.SetContentType(r.Header.Get("Content-Type"))
			metadata.MarkUpdated()
			return c.PutMetadata(key, metadata, tx)
		})
		if err != nil {
			log.Printf("Error persisting data: %v", err)
			http.Error(w, "Could not persist data", http.StatusInternalServerError)
			return
		}
	} else if r.Method == http.MethodDelete {
		// auth check: disallow public deletes
		if _, ok := user.(*AnonymousUser); ok {
			log.Println("Unauthorized delete attempt")
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized Writer", http.StatusUnauthorized)
			return
		}

		err := c.db.Update(func(tx *bolt.Tx) error {
			metadata := c.GetMetadata(key, tx)

			// auth check: writer allowlist
			if !user.InGroup(metadata.Writers) {
				log.Println("Unauthorized delete attempt")
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
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
		log.Printf("Invalid action for key: %s", key)
		fmt.Fprintf(w, "Invalid action for key: %s", key)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
