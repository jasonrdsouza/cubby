package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	serveTrustedOrigins := serveCmd.String("trusted-origins", "", "comma-separated origins (eg. https://app.example.com) whose pages may make cross-origin writes from a browser")

	credentialsCmd := flag.NewFlagSet("credentials", flag.ExitOnError)
	credentialsDbFile := credentialsCmd.String("path", "cubby.db", "filepath where cubby data is stored")

	addAdminCmd := flag.NewFlagSet("addadmin", flag.ExitOnError)
	addAdminDbFile := addAdminCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	addAdminName := addAdminCmd.String("name", "", "admin username to add (or whose password to reset)")
	addAdminPassword := addAdminCmd.String("password", "", "password for this admin")

	removeAdminCmd := flag.NewFlagSet("removeadmin", flag.ExitOnError)
	removeAdminDbFile := removeAdminCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	removeAdminName := removeAdminCmd.String("name", "", "admin to remove")

	addReaderCmd := flag.NewFlagSet("addreader", flag.ExitOnError)
	addReaderDbFile := addReaderCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	addReaderName := addReaderCmd.String("name", "", "reader key name to add")
	addReaderRegenerate := addReaderCmd.Bool("regenerate", false, "replace the secret of an existing reader key")

	removeReaderCmd := flag.NewFlagSet("removereader", flag.ExitOnError)
	removeReaderDbFile := removeReaderCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	removeReaderName := removeReaderCmd.String("name", "", "reader key to remove (it is also removed from every blob that lists it)")

	compactCmd := flag.NewFlagSet("compact", flag.ExitOnError)
	compactDbFile := compactCmd.String("path", "cubby.db", "filepath where cubby data is stored")
	compactOut := compactCmd.String("o", "", "filepath to write the compacted copy to (must not exist)")

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

		fmt.Fprint(os.Stderr, " credentials:\n")
		credentialsCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " addadmin:\n")
		addAdminCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " removeadmin:\n")
		removeAdminCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " addreader:\n")
		addReaderCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " removereader:\n")
		removeReaderCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " compact:\n")
		compactCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " get:\n")
		getCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " put:\n")
		putCmd.PrintDefaults()

		fmt.Fprint(os.Stderr, " remove:\n")
		removeCmd.PrintDefaults()
	}

	if len(os.Args) < 2 {
		fmt.Println("Please specify subcommand (serve, credentials, addadmin, removeadmin, addreader, removereader, compact, get, put, remove)")
		flag.Usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd.Parse(os.Args[2:])
		startServer(*servePort, *serveFile, *serveMaxSize, *serveTrustedOrigins)
	case "credentials":
		credentialsCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*credentialsDbFile)
		var admins, readers []string
		for _, user := range cubbyServer.ListCredentials() {
			if user.Admin {
				admins = append(admins, user.Name())
			} else {
				readers = append(readers, user.Name())
			}
		}
		printNames("Admins", admins)
		printNames("Reader keys", readers)
	case "addadmin":
		addAdminCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*addAdminDbFile)
		err := cubbyServer.AddAdmin(*addAdminName, *addAdminPassword)
		if err != nil {
			log.Fatal(err)
		}
	case "removeadmin":
		removeAdminCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*removeAdminDbFile)
		if err := cubbyServer.RemoveAdmin(*removeAdminName); err != nil {
			log.Fatal(err)
		}
	case "addreader":
		addReaderCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*addReaderDbFile)
		addOrRegenerate := cubbyServer.AddReaderKey
		if *addReaderRegenerate {
			addOrRegenerate = cubbyServer.RegenerateReaderKey
		}
		secret, err := addOrRegenerate(*addReaderName)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Reader key %s: %s\n", *addReaderName, secret)
		fmt.Println("Use it as Basic Auth username:password. The secret is not shown again.")
	case "removereader":
		removeReaderCmd.Parse(os.Args[2:])
		cubbyServer := adminServer(*removeReaderDbFile)
		if err := cubbyServer.RemoveReaderKey(*removeReaderName); err != nil {
			log.Fatal(err)
		}
	case "compact":
		compactCmd.Parse(os.Args[2:])
		if *compactOut == "" {
			log.Fatal("compact: -o is required")
		}
		before, after, err := CompactDB(*compactDbFile, *compactOut)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Compacted %s (%s) into %s (%s).\n", *compactDbFile, formatBytes(before), *compactOut, formatBytes(after))
		fmt.Println("With the server stopped, replace the original with the compacted copy to use it.")
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
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		flag.Usage()
		os.Exit(1)
	}
}

func printNames(heading string, names []string) {
	fmt.Printf("%s:\n", heading)
	if len(names) == 0 {
		fmt.Println("  (none)")
	}
	for _, name := range names {
		fmt.Printf("- %s\n", name)
	}
}

func adminServer(dbPath string) *CubbyServer {
	cubby, err := NewCubbyServer(dbPath, 1) // maxObjectSize doesn't matter here
	if err != nil {
		log.Fatal(err)
	}
	return cubby
}

func startServer(port int, dbPath string, maxObjectSizeMB int, trustedOrigins string) {
	cubby, err := NewCubbyServer(dbPath, maxObjectSizeMB)
	if err != nil {
		log.Fatal(err)
	}
	defer cubby.Close()

	var origins []string
	if trustedOrigins != "" {
		origins = strings.Split(trustedOrigins, ",")
	}
	handler, err := cubby.HTTPHandler(origins)
	if err != nil {
		log.Fatal(err)
	}
	addr := ":" + strconv.Itoa(port)
	log.Printf("Starting cubby server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func initClient(serverAddr string) *CubbyClient {
	client, err := NewCubbyClient(serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	return client
}
