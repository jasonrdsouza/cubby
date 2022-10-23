## Cubby
A simple, web-native object store.

### Building
For current system:
```bash
go fmt && goimports -w . && and go build -o bin/cubby
```

For alternate platforms (ie. linux):
```bash
GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux
```

### Releasing
Build binaries, then tag and push the release. Out of band, upload the newly generated binaries:
```bash
export CUBBY_VERSION=0.2

go fmt && goimports -w . && go build -o bin/cubby-darwin && GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux && git tag v$CUBBY_VERSION && git push origin v$CUBBY_VERSION
```

### Deploying
Fetch latest version, or whichever version is specified in `CUBBY_VERSION`
```bash
export CUBBY_VERSION=0.2

wget -O /tmp/cubby https://github.com/jasonrdsouza/cubby/releases/download/v$CUBBY_VERSION/cubby-linux
sudo mv /tmp/cubby /usr/local/bin/cubby
chmod +x /usr/local/bin/cubby
sudo systemctl restart cubby
```

### Usage
Run the server:
```bash
./bin/cubby-darwin serve -path data/cubby.db
```

Get data (using [httpie](https://httpie.io/))
```bash
http GET localhost:8080/test
```

Put JSON data
```bash
http POST http://localhost:8080/test key=value
```

Put other data types (specifying content type)
```bash
http POST http://localhost:8080/pdf Content-Type:application/pdf < ~/Downloads/test.pdf
```

### Todo
https://trello.com/c/A9108x0T

