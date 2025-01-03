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
	DEFAULT_ADDR string = "http://localhost:8383"
)

func main() {
	SetENV()

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	servePort := serveCmd.Int("port", 8383, "port to serve on")
	serveFile := serveCmd.String("path", "cubby.db", "filepath to store cubby data at")
	serveMaxSize := serveCmd.Int("max", 10, "max cubby object size in MB")

	listUserCmd := flag.NewFlagSet("listusers", flag.ExitOnError)
	listUserDbFile := listUserCmd.String("path", "cubby.db", "filepath where cubby data is stored")

	addUserCmd := flag.NewFlagSet("adduser", flag.ExitOnError)
	addUserDbFile := addUserCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	addUserName := addUserCmd.String("name", "", "username to add")
	addUserPassword := addUserCmd.String("password", "", "password for this user")
	addUserAdmin := addUserCmd.Bool("admin", false, "whether to make this user an admin or not")

	removeUserCmd := flag.NewFlagSet("removeuser", flag.ExitOnError)
	removeUserDbFile := removeUserCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	removeUserName := removeUserCmd.String("name", "", "username to remove")

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

		fmt.Fprint(os.Stderr, " listusers:\n")
		listUserCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " adduser:\n")
		addUserCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " removeuser:\n")
		removeUserCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " get:\n")
		getCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " put:\n")
		putCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " remove:\n")
		removeCmd.PrintDefaults()
	}

	if len(os.Args) < 2 {
		fmt.Println("Please specify subcommand (serve, listusers, adduser, removeuser, get, put, remove)")
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd.Parse(os.Args[2:])
		startServer(*servePort, *serveFile, *serveMaxSize)
	case "listusers":
		listUserCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*listUserDbFile)
		users := cubbyServer.ListUsers()
		if len(users) == 0 {
			fmt.Println("No users found")
		} else {
			fmt.Println("Users:")
			for _, user := range users {
				fmt.Printf("- %s\n", user)
			}
		}
	case "adduser":
		addUserCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*addUserDbFile)
		err := cubbyServer.AddUser(*addUserName, *addUserPassword, *addUserAdmin)
		if err != nil {
			log.Fatal(err)
		}
	case "removeuser":
		removeUserCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*removeUserDbFile)
		err := cubbyServer.RemoveUser(*removeUserName)
		if err != nil {
			log.Fatal(err)
		}
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

func adminServer(dbPath string) *CubbyServer {
	cubby, err := NewCubbyServer(dbPath, 1) // maxObjectSize doesn't matter here
	if err != nil {
		log.Fatal(err)
	}
	return cubby
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
