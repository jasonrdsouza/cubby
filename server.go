package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"text/template"
	"time"

	"github.com/boltdb/bolt"
)

const (
	DB_BUCKET string = "MyBucket" // todo: change me to avoid data loss for new major version
)

type CubbyServer struct {
	filename      string
	dataBucket    string
	metaBucket    string
	usersBucket   string
	db            *bolt.DB
	maxObjectSize int64
	log           *log.Logger
	indexTemplate *template.Template
}

func NewCubbyServer(dbFilename string, maxObjectSizeMB int) (*CubbyServer, error) {
	server := &CubbyServer{
		filename:      dbFilename,
		dataBucket:    DB_BUCKET,
		metaBucket:    DB_BUCKET + "_metadata",
		usersBucket:   USERS_BUCKET,
		maxObjectSize: int64(maxObjectSizeMB * 1024 * 1024),
		log:           log.Default(),
		indexTemplate: IndexTemplate(),
	}

	db, err := bolt.Open(server.filename, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		server.log.Printf("Error opening database: %v", err)
		return nil, err
	}

	server.db = db
	err = server.initialize()
	if err != nil {
		server.log.Printf("Error initializing database: %v", err)
		return nil, err
	}

	server.log.Println("Successfully initialized cubby server")
	return server, nil
}

func (c *CubbyServer) initialize() error {
	return c.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(c.dataBucket))
		if err != nil {
			return fmt.Errorf("DB create data bucket: %s", err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(c.metaBucket))
		if err != nil {
			return fmt.Errorf("DB create meta bucket: %s", err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(c.usersBucket))
		if err != nil {
			return fmt.Errorf("DB create users bucket: %s", err)
		}

		return nil
	})
}

func (c *CubbyServer) Close() {
	c.log.Println("Spinning down cubby server")
	c.db.Close()
}

func (c *CubbyServer) Version() string {
	return BuiltGitCommit
}

func (c *CubbyServer) GetMetadata(key string, tx *bolt.Tx) *CubbyMetadata {
	b := tx.Bucket([]byte(c.metaBucket))
	v := b.Get([]byte(key))

	decoder := gob.NewDecoder(bytes.NewBuffer(v))
	var metadata CubbyMetadata
	err := decoder.Decode(&metadata)
	if err != nil {
		c.log.Printf("Error decoding metadata for key: %v. %v", key, err)
	}

	// In either case, return the metadata struct. Either it will be empty, or
	// it will contain the requested metadata
	return &metadata
}

func (c *CubbyServer) GetAtomic(key string) string {
	var value []byte
	c.db.View(func(tx *bolt.Tx) error {
		value = c.Get(key, tx)
		return nil
	})

	c.log.Printf("Successfully got key: %s", key)
	return string(value)
}

func (c *CubbyServer) Get(key string, tx *bolt.Tx) []byte {
	b := tx.Bucket([]byte(c.dataBucket))
	v := b.Get([]byte(key))

	// v is only valid for the duration of the transaction, so copy the value
	// to a new byte array for use later on
	var value []byte
	value = append(value, v...)
	return value
}

func (c *CubbyServer) PutMetadata(key string, metadata *CubbyMetadata, tx *bolt.Tx) error {
	b := tx.Bucket([]byte(c.metaBucket))

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(metadata)
	if err != nil {
		c.log.Printf("Error encoding metadata for key: %s", key)
		return err
	}

	err = b.Put([]byte(key), buf.Bytes())
	if err != nil {
		c.log.Printf("Error putting metadata for key: %s", key)
	} else {
		c.log.Printf("Successfully put metadata for key: %s", key)
	}
	return err
}

func (c *CubbyServer) PutAtomic(key, value string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		return c.Put(key, []byte(value), tx)
	})

	if err != nil {
		c.log.Printf("Error putting key: %s", key)
	} else {
		c.log.Printf("Successfully put key: %s", key)
	}
	return err
}

func (c *CubbyServer) Put(key string, value []byte, tx *bolt.Tx) error {
	b := tx.Bucket([]byte(c.dataBucket))
	err := b.Put([]byte(key), value)
	return err
}

func (c *CubbyServer) RemoveMetadata(key string, tx *bolt.Tx) error {
	b := tx.Bucket([]byte(c.metaBucket))
	err := b.Delete([]byte(key))
	if err != nil {
		c.log.Printf("Error removing metadata for key: %s", key)
	} else {
		c.log.Printf("Successfully removed metadata for key: %s", key)
	}
	return err
}

func (c *CubbyServer) RemoveAtomic(key string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		return c.Remove(key, tx)
	})

	if err != nil {
		c.log.Printf("Error removing key: %s", key)
	} else {
		c.log.Printf("Successfully removed key: %s", key)
	}
	return err
}

func (c *CubbyServer) Remove(key string, tx *bolt.Tx) error {
	b := tx.Bucket([]byte(c.dataBucket))
	err := b.Delete([]byte(key))
	return err
}

func (c *CubbyServer) ListAtomic() []string {
	var keys []string
	c.db.View(func(tx *bolt.Tx) error {
		keys = c.List(tx)
		return nil
	})

	c.log.Printf("Successfully listed keys")
	return keys
}

func (c *CubbyServer) List(tx *bolt.Tx) []string {
	b := tx.Bucket([]byte(c.dataBucket))
	cursor := b.Cursor()

	keys := []string{}
	for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
		keys = append(keys, string(k))
	}

	return keys
}
