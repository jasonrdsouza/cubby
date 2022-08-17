package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boltdb/bolt"
)

const (
	DB_BUCKET    string = "MyBucket"
	DEFAULT_ADDR string = "http://localhost:8080"
)

func main() {
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	servePort := serveCmd.Int("port", 8080, "port to serve on")
	serveFile := serveCmd.String("path", "cubby.db", "filepath to store cubby data at")
	serveMaxSize := serveCmd.Int("max", 10, "max cubby object size in MB")

	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	getAddr := getCmd.String("addr", DEFAULT_ADDR, "cubby server address")
	getKey := getCmd.String("key", "", "key to get")

	putCmd := flag.NewFlagSet("put", flag.ExitOnError)
	putAddr := putCmd.String("addr", DEFAULT_ADDR, "cubby server address")
	putKey := putCmd.String("key", "", "key to put")
	putValue := putCmd.String("value", "", "value to put")

	removeCmd := flag.NewFlagSet("remove", flag.ExitOnError)
	removeAddr := removeCmd.String("addr", DEFAULT_ADDR, "cubby server address")
	removeKey := removeCmd.String("key", "", "key to remove")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])

		fmt.Fprint(os.Stderr, " serve:\n")
		serveCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " get:\n")
		getCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " put:\n")
		putCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " remove:\n")
		removeCmd.PrintDefaults()
	}

	if len(os.Args) < 2 {
		fmt.Println("Please specify subcommand (serve, get, put, remove)")
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd.Parse(os.Args[2:])
		startServer(*servePort, *serveFile, *serveMaxSize)
	case "get":
		getCmd.Parse(os.Args[2:])
		client := initClient(*getAddr)
		value, err := client.Get(*getKey)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(value)
	case "put":
		putCmd.Parse(os.Args[2:])
		client := initClient(*putAddr)
		if err := client.Put(*putKey, *putValue); err != nil {
			log.Fatal(err)
		}
	case "remove":
		removeCmd.Parse(os.Args[2:])
		client := initClient(*removeAddr)
		if err := client.Remove(*removeKey); err != nil {
			log.Fatal(err)
		}
	}
}

func startServer(port int, dbPath string, maxObjectSizeMB int) {
	cubby, err := NewCubbyServer(dbPath, maxObjectSizeMB)
	if err != nil {
		log.Fatal(err)
	}
	defer cubby.Close()

	http.HandleFunc("/", cubby.Handler)
	addr := ":" + strconv.Itoa(port)
	log.Printf("Starting cubby server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func initClient(serverAddr string) *CubbyClient {
	client, err := NewCubbyClient(serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

type CubbyClient struct {
	serverAddr *url.URL
	httpClient *http.Client
}

func NewCubbyClient(serverAddr string) (*CubbyClient, error) {
	parsedAddr, err := url.Parse(serverAddr)
	if err != nil {
		return nil, err
	}
	return &CubbyClient{serverAddr: parsedAddr, httpClient: &http.Client{}}, nil
}

func (c *CubbyClient) keyUrlString(key string) string {
	return c.serverAddr.JoinPath(key).String()
}

func (c *CubbyClient) validate(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed with status code %v", resp.StatusCode)
	}
	return resp, nil
}

func (c *CubbyClient) Get(key string) (string, error) {
	resp, err := c.validate(c.httpClient.Get(c.keyUrlString(key)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func (c *CubbyClient) Put(key, value string) error {
	_, err := c.validate(c.httpClient.Post(c.keyUrlString(key), "", strings.NewReader(value)))
	return err
}

func (c *CubbyClient) Remove(key string) error {
	request, err := http.NewRequest(http.MethodDelete, c.keyUrlString(key), nil)
	if err != nil {
		return err
	}
	_, err = c.validate(c.httpClient.Do(request))
	return err
}

type CubbyServer struct {
	filename      string
	bucket        string
	db            *bolt.DB
	maxObjectSize int64
	log           *log.Logger
}

func NewCubbyServer(dbFilename string, maxObjectSizeMB int) (*CubbyServer, error) {
	server := &CubbyServer{
		filename:      dbFilename,
		bucket:        DB_BUCKET,
		maxObjectSize: int64(maxObjectSizeMB * 1024 * 1024),
		log:           log.Default(),
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
		_, err := tx.CreateBucketIfNotExists([]byte(c.bucket))
		if err != nil {
			return fmt.Errorf("DB create bucket: %s", err)
		}
		return nil
	})
}

func (c *CubbyServer) Close() {
	c.log.Println("Spinning down cubby server")
	c.db.Close()
}

func (c *CubbyServer) Get(key string) string {
	var value []byte
	c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.bucket))
		v := b.Get([]byte(key))

		// v is only valid for the duration of the transaction, so copy the value
		// to a new byte array for use later on
		value = append(value, v...)
		return nil
	})

	c.log.Printf("Successfully got key: %s", key)
	return string(value)
}

func (c *CubbyServer) Put(key, value string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.bucket))
		err := b.Put([]byte(key), []byte(value))
		return err
	})

	if err != nil {
		c.log.Printf("Error putting key: %s", key)
	} else {
		c.log.Printf("Successfully put key: %s", key)
	}
	return err
}

func (c *CubbyServer) Remove(key string) error {
	err := c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(c.bucket))
		err := b.Delete([]byte(key))
		return err
	})

	if err != nil {
		c.log.Printf("Error removing key: %s", key)
	} else {
		c.log.Printf("Successfully removed key: %s", key)
	}
	return err
}

func (c *CubbyServer) Handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "" {
		fmt.Fprintf(w, "Specify key")
		return
	}

	key := r.URL.Path[1:]

	if r.Method == http.MethodGet {
		fmt.Fprint(w, c.Get(key))
	} else if r.Method == http.MethodPost {
		var b bytes.Buffer
		r.Body = http.MaxBytesReader(w, r.Body, c.maxObjectSize)
		_, err := b.ReadFrom(r.Body)
		if err != nil {
			http.Error(w, "Could not read data", http.StatusInternalServerError)
			return
		}

		err = c.Put(key, b.String())
		if err != nil {
			http.Error(w, "Could not persist data", http.StatusInternalServerError)
			return
		}

	} else if r.Method == http.MethodDelete {
		err := c.Remove(key)
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
