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
Build binaries, then tag and push the release. Out of band, make the Github release, and upload the newly generated binaries:
```bash
export CUBBY_VERSION=0.2

go fmt && goimports -w . && go build -o bin/cubby-darwin && GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux

git tag v$CUBBY_VERSION && git push origin v$CUBBY_VERSION
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

Note that this assumes you are using [systemd](https://en.wikipedia.org/wiki/Systemd) as the service daemon manager. See an example service definition file at [cubby.service](cubby.service). It must be copied to `/lib/systemd/system/cubby.service`, updated as necessary depending on the user, paths, ports, etc that you wish to configure, and can then be used as follows:

```bash

# enable Cubby in SystemCTL so that it starts on boot
sudo systemctl enable cubby

# check the service status
sudo systemctl status cubby

# start the service if not running
sudo systemctl start cubby

# restart service if it is running
sudo systemctl restart cubby
```


### Usage
Run the server:
```bash
./bin/cubby-darwin serve -path data/cubby.db
```

Get data (using [httpie](https://httpie.io/))
```bash
http GET http://localhost:8383/test
```

Put JSON data
```bash
http POST http://localhost:8383/test key=value
```

Put other data types (specifying content type)
```bash
# pdf
http POST http://localhost:8383/test.pdf Content-Type:application/pdf < ~/Downloads/test.pdf

# png
http POST http://localhost:8383/screenshot.png Content-Type:image/png < screenshot.png
```

Delete data
```bash
http DELETE http://localhost:8383/test
```

Specify credentials (basic auth)
```bash
# this example is of a delete, but the same params can be used with any HTTP verb
http DELETE http://localhost:8383/test -a username:password
```


### Todo
https://trello.com/c/A9108x0T

