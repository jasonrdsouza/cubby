## Cubby
A simple, web-native object store.

### Building
For MacOS:
```bash
go build -o bin/cubby
```

For Linux:
```bash
GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux
```

### Todo
- better bucket support (with default bucket)
- integration of other BoltDB functionality
- content type support (via headers, stored in a "metadata" bucket or key?)

