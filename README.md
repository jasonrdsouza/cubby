# Cubby
A simple, web-native object store.

## Usage

### Deploying
Fetch [latest release version](https://github.com/jasonrdsouza/cubby/releases), or whichever version is specified in `CUBBY_VERSION`:

```bash
export CUBBY_VERSION=1.0

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

### Authenticating
By default, Cubby stores all of its data in a single [BoltDB](https://github.com/boltdb/bolt) file (canonically called `caddy.db`). Before exposing the service to the world, you must create users to authenticate requests against. Subsequent Cubby operations (particularly "write" operations) will only succeed with valid user credentials.

Adding users can be accomplished via the `cubby adduser` command as follows:

```bash
# regular user
./bin/cubby adduser -path data/cubby.db -name username -password password

# admin user
./bin/cubby adduser -path data/cubby.db -name username -password password -admin
```

Removing users is similarly straightforward:

```bash
./bin/cubby removeuser -path data/cubby.db -name username
```

Finally, listing the existing users can be done as follows:

```bash
./bin/cubby listusers -path data/cubby.db
```

All user operations require direct access to the underlying `caddy.db` file.

#### Transport Security
**Note that Cubby itself does not provide transport level security. It is up to the system administrator to ensure that Cubby is only accessible via a secure channel (ie. HTTPS).** The easiest way to accomplish this is to use a reverse proxy like [NGINX](https://www.nginx.com/) or [Caddy](https://caddyserver.com/).

User auth is accomplished via HTTP basic auth ([hence the need for transport level security](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication#security_of_basic_authentication)), and so should work with myriad web-native tooling (eg. browsers, curl, httpie, etc).

#### Authorization
Right now the AuthZ model Cubby maintains is very basic. There are 3 possible reader/ writer groups: Admin, User, and Public.

- Admins have access to do everything
- Users have access to read/write all cubbies which are not admin protected
- Public has access to read any public cubbies, but can not perform writes.


### Running
Run the server
```bash
./bin/cubby-darwin serve -path data/cubby.db
```

Navigate to http://localhost:8383/ to view the Cubby UI, which shows a listing of all "occupied" cubbies, as well as the version of Cubby that is running.

Put JSON data (using [httpie](https://httpie.io/)), specifying user credentials since writes are limited to authenticated users
```bash
http POST http://localhost:8383/test key=value -a username:password
```

Get data
```bash
http GET http://localhost:8383/test
```

Get data and specify credentials (basic auth)
```bash
# this example is of a GET, but the same params can be used with any HTTP verb
http GET http://localhost:8383/test -a username:password
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

Download large file
```bash
http --download https://localhost:8383/largeFile.tar.gz
```

Upload a favicon (specifying the right content type). Given the web-native way Cubby works, if you specify the key as `favicon.ico`, Cubby will automatically serve this file whenever a page is requested by a browser.
```bash
http -a username:password POST http://localhost:8383/favicon.ico 'Content-Type:image/x-icon' < ~/Downloads/cubby.ico
```

Upload data that can only be read by other authorized users (ie. not publicly accessible) via the `X-CUBBY-READER` header
```bash
# data=confidential is the json payload
# X-CUBBY-READER:user is the header that specifies that only authenticated users can read this data
http -a username:password POST localhost:8383/authTest data=confidential X-CUBBY-READER:user
```


## Development

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
export CUBBY_VERSION=1.0

go fmt && goimports -w . && go build -o bin/cubby-darwin && GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux

git tag v$CUBBY_VERSION && git push origin v$CUBBY_VERSION
```


### Todo
https://trello.com/c/A9108x0T

