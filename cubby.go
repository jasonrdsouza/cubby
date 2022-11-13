package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

const (
	DB_BUCKET    string = "MyBucket"
	DEFAULT_ADDR string = "http://localhost:8383"
)

func main() {
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	servePort := serveCmd.Int("port", 8383, "port to serve on")
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
