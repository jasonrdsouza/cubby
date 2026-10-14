# Cubby
A simple, web-native object store.

## Usage

### Deploying
[`deploy.sh`](deploy.sh) installs or upgrades Cubby from a [GitHub release](https://github.com/jasonrdsouza/cubby/releases). It detects the architecture, verifies the download against the release's `SHA256SUMS`, keeps the previous binary as `cubby.prev`, restarts the service, and rolls back automatically if the server doesn't respond afterwards.

```bash
# install the latest release
curl -fsSL https://raw.githubusercontent.com/jasonrdsouza/cubby/main/deploy.sh | bash

# or keep a copy on the server
curl -fsSLO https://raw.githubusercontent.com/jasonrdsouza/cubby/main/deploy.sh && chmod +x deploy.sh
./deploy.sh              # latest release
./deploy.sh v1.6         # a specific release
./deploy.sh --rollback   # restore the previously installed binary
```

Settings are environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `CUBBY_BIN` | `/usr/local/bin/cubby` | where the binary is installed |
| `CUBBY_SERVICE` | `cubby` | systemd unit to restart |
| `CUBBY_HEALTH_URL` | `http://localhost:8383/` | checked after the restart; set it to match your `-port` |
| `CUBBY_DB` | unset | if set, the DB is copied to `$CUBBY_DB.bak` (with the service stopped) before the new version starts |

For example, with the sample [cubby.service](cubby.service):
```bash
CUBBY_HEALTH_URL=http://localhost:8081/ CUBBY_DB=/var/data/cubby.db ./deploy.sh
```

Checksums are published from v1.6 onwards; to install an older release, download it manually from the releases page.

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
By default, Cubby stores all of its data in a single [BoltDB](https://github.com/boltdb/bolt) file (canonically called `cubby.db`). Before exposing the service to the world, create an admin. Only admins can write.

```bash
# add an admin (or reset an existing admin's password)
./bin/cubby addadmin -path data/cubby.db -name username -password password

# add a reader key; its secret is generated and printed once
./bin/cubby addkey -path data/cubby.db -name family

# replace a reader key's secret, invalidating the old one
./bin/cubby addkey -path data/cubby.db -name family -regenerate

# list admins and reader keys
./bin/cubby listusers -path data/cubby.db

# remove an admin or reader key
./bin/cubby removeuser -path data/cubby.db -name family
```

These commands require direct access to the underlying `cubby.db` file, and the server must be stopped while they run (BoltDB allows one process at a time).

#### Transport Security
**Note that Cubby itself does not provide transport level security. It is up to the system administrator to ensure that Cubby is only accessible via a secure channel (ie. HTTPS).** The easiest way to accomplish this is to use a reverse proxy like [NGINX](https://www.nginx.com/) or [Caddy](https://caddyserver.com/).

User auth is accomplished via HTTP basic auth ([hence the need for transport level security](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication#security_of_basic_authentication)), and so should work with myriad web-native tooling (eg. browsers, curl, httpie, etc).

#### Cross-Site Request Forgery
Browsers resend Basic Auth credentials automatically, so Cubby rejects writes (POST/DELETE) that a browser makes from a page on another origin, with `403 Forbidden`. Reads, same-origin pages (including Cubby's own UI), and non-browser clients like curl are unaffected. To let a web app on another origin write from the browser, trust its origin explicitly:

```bash
./bin/cubby serve -path data/cubby.db -trusted-origins https://app.example.com
```

#### Authorization
Every blob is either public or private:

- **Public** blobs can be read by anyone. New blobs are public unless the write says otherwise.
- **Private** blobs can be read by admins, plus any reader keys the blob lists.
- **Admins** can read and write everything. Writes and deletes always require an admin.

A reader key is a read-only credential that you can hand out in lower-trust places: if it leaks, it exposes only the blobs that list it, and it can never write. Use it as the Basic Auth username and password (`family:<secret>`). Removing a key also removes it from every blob that listed it.

Access is set with the `X-Cubby-Reader` header when writing:

| `X-Cubby-Reader` | Who can read |
|---|---|
| `public` | anyone |
| `private` | admins only |
| `family,work` | admins, plus the `family` and `work` reader keys |
| *(omitted)* | unchanged for an existing blob; public for a new one |


### Running
Run the server
```bash
./bin/cubby-darwin serve -path data/cubby.db
```

Navigate to http://localhost:8383/ to view the Cubby UI, which lists the "occupied" cubbies you can read (send credentials to see private ones), as well as the version of Cubby that is running.

Put JSON data (using [httpie](https://httpie.io/)), specifying admin credentials since writes are limited to admins
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
http -a username:password DELETE http://localhost:8383/test
```

Download large file
```bash
http --download https://localhost:8383/largeFile.tar.gz
```

Upload a favicon (specifying the right content type). Given the web-native way Cubby works, if you specify the key as `favicon.ico`, Cubby will automatically serve this file whenever a page is requested by a browser.
```bash
http -a username:password POST http://localhost:8383/favicon.ico 'Content-Type:image/x-icon' < ~/Downloads/cubby.ico
```

Upload data that only the `family` reader key (and admins) can read, via the `X-Cubby-Reader` header
```bash
# data=confidential is the json payload
http -a username:password POST localhost:8383/authTest data=confidential X-Cubby-Reader:family

# read it with the reader key
http -a family:<secret> GET localhost:8383/authTest
```


## Development

### Building
For current system:
```bash
go fmt && go tool goimports -w . && go vet && go build -o bin/cubby
```

For alternate platforms (ie. linux):
```bash
GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux
```

### Releasing
Tag a commit on `main` and push the tag. The [release workflow](.github/workflows/release.yml) builds binaries for Linux, macOS and Windows, publishes them with a `SHA256SUMS` file, and creates the GitHub release with generated notes:
```bash
export CUBBY_VERSION=1.6

go vet ./... && go test ./...
git tag v$CUBBY_VERSION && git push origin v$CUBBY_VERSION
```


### Todo
https://trello.com/c/A9108x0T

