# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cubby is a simple, web-native object store written in Go. It provides a REST API for storing and retrieving arbitrary data (JSON, binary files, images, etc.) with Basic Auth: public blobs, private blobs readable by per-blob reader keys, and admins who are the only writers. All data is persisted in a single BoltDB file.

## Development Commands

### Building

Build for current platform:
```bash
go fmt && go tool goimports -w . && go vet && go build -o bin/cubby
```

Build for Linux (cross-compile):
```bash
GOOS=linux GOARCH=amd64 go build -o bin/cubby-linux
```

Build for macOS (cross-compile):
```bash
go build -o bin/cubby-darwin
```

### Running the Server

Start the server locally:
```bash
./bin/cubby-darwin serve -path data/cubby.db
```

Default port is 8383. Access the UI at http://localhost:8383/

### User Management

All user commands require direct access to the BoltDB file (the server must be stopped):

```bash
# Add admin (or reset an admin's password)
./bin/cubby addadmin -path data/cubby.db -name username -password password

# Add reader key (prints its generated secret once); -regenerate replaces the secret
./bin/cubby addkey -path data/cubby.db -name family

# List admins and reader keys
./bin/cubby listusers -path data/cubby.db

# Remove an admin or reader key (also unlists a reader key from every blob)
./bin/cubby removeuser -path data/cubby.db -name family
```

### Client Commands

The binary includes a built-in client:
```bash
# Get data
./bin/cubby get -addr http://localhost:8383 -key mykey

# Put data
./bin/cubby put -addr http://localhost:8383 -key mykey -value myvalue

# Remove data
./bin/cubby remove -addr http://localhost:8383 -key mykey
```

Client credentials can be set via environment variables:
- `CUBBY_USERNAME`
- `CUBBY_PASSWORD`

## Architecture

### Single-Package Design

The entire application is in a single Go package (main) with no subdirectories. All `.go` files are at the root level.

### Core Components

**Server (`server.go`)**
- `CubbyServer` struct manages the BoltDB connection and operations
- Uses 3 BoltDB buckets: data bucket (actual values), metadata bucket (permissions/content-type), users bucket (auth)
- Provides atomic and transactional operations for Get/Put/Remove/List
- Max object size is configurable (default 10MB)

**HTTP Handler (`http.go`)**
- Single `Handler` function processes all HTTP requests
- `HTTPHandler()` wraps it in `http.CrossOriginProtection` (CSRF: browsers resend cached Basic Auth), so browser writes from other origins get 403 unless listed in `serve -trusted-origins`; tests use `HTTPHandler` too
- Sets CORS headers (`Access-Control-Allow-Origin: *`) on all responses and handles OPTIONS preflight
- Serves index page at `/` listing the occupied keys the requester can read
- Serves the embedded JavaScript client at `/client.js`
- All other paths are treated as keys: `/:key`
- Supports GET, HEAD, POST (for writes), DELETE methods
- Raw GETs are served via `http.ServeContent` with a strong ETag derived from `metadata.UpdatedAt` (`CubbyMetadata.ETag()`), so conditional GETs and Range work; themed views get no ETag
- POST/DELETE honor `If-Match` / `If-None-Match` (`preconditions.go`), evaluated after auth and inside the same bolt Update transaction as the write
- Reads are checked with `User.CanRead(metadata)`; POST/DELETE require `User.IsAdmin()`

**Authentication & Authorization (`users.go`, `admin.go`, `metadata.go`)**
- HTTP Basic Auth for all authenticated operations
- Stored credentials (`RegularUser`, users bucket) are either admins (`Admin: true`) or reader keys (`Admin: false`)
- Passwords are bcrypt-hashed with cost 8; reader key secrets are generated (`crypto/rand.Text`)
- Unauthenticated requests (or bad credentials) get an `AnonymousUser`
- Admins read and write everything; reader keys read public blobs and private blobs that list them; anonymous reads public blobs only. Only admins write or delete
- Per-blob read access is set with the `X-Cubby-Reader` header on POST: `public`, `private` (admins only), or a comma-separated list of reader key names (validated against existing reader keys). Omitted keeps the existing access, or public for a new blob
- `RemoveUser` also strips a removed reader key from every blob's `ReaderKeys`, so a recreated key of the same name inherits nothing

**Metadata (`metadata.go`)**
- Each key has associated metadata: ContentType, UpdatedAt, Public, ReaderKeys
- Metadata is stored separately from data using gob encoding

**Admin Page (`adminpage.go`, `admin.html`)**
- Served under the reserved `/_admin` prefix (`isAdminPath`); every request there requires an admin (401 + Basic Auth challenge otherwise) and is routed by `adminMux`, a method-pattern `http.ServeMux`
- Plain HTML forms, no JS beyond `confirm()`: create/regenerate/delete reader keys, change a blob's access (`SetAccess`, which leaves content and ETag untouched), delete blobs. Admins are CLI-only
- Successful actions 303 back to `/_admin`; a generated secret is rendered directly (never in a URL) with `Cache-Control: no-store`
- CSRF is covered by `HTTPHandler`'s `CrossOriginProtection`

**Go Client (`client.go`)**
- Simple HTTP client for programmatic access from Go
- Reads credentials from `CUBBY_USERNAME` and `CUBBY_PASSWORD` environment variables

**JavaScript Client (`js-client/`)**
- Browser-based client for use in HTML/CSS/JS web apps
- Provides a `Cubby` class with `getState()`/`setState()` API for managing a single JSON state object
- Persists state to browser localStorage with optional background sync to a Cubby server
- Sync strategy: debounced push after changes + periodic interval as safety net
- Conflict resolution: last-write-wins using timestamps
- Graceful degradation: works with just localStorage if no server is configured
- Server config (including credentials) is persisted to localStorage by default for reuse across page loads
- The JS file is embedded into the Go binary via `//go:embed` in `templates.go` and served at `/client.js`

**Entry Point (`cubby.go`)**
- CLI uses flag.NewFlagSet for subcommands
- Main subcommands: serve, listusers, addadmin, addkey, removeuser, get, put, remove

### Data Storage

- **Database**: BoltDB (embedded key-value store)
- **Location**: Single file specified by `-path` flag (default: `cubby.db`)
- **Buckets**:
  - Data bucket: stores actual key-value data
  - Metadata bucket: stores CubbyMetadata structs (gob-encoded)
  - Users bucket: stores RegularUser structs (gob-encoded)

### Key Design Patterns

1. **Transactional Operations**: Most operations use BoltDB transactions (`db.View` for reads, `db.Update` for writes). The server provides both atomic operations (handle transaction internally) and transaction-aware operations (accept `*bolt.Tx` parameter).

2. **Authorization Flow**:
   - User authenticated via HTTP Basic Auth → `FetchUser()` returns `User` interface
   - Reads: metadata retrieved for the key, `User.CanRead(metadata)` checks permission
   - Writes: `User.IsAdmin()`, checked before the body is read

3. **Web-Native**: The server serves content with proper Content-Type headers, making objects directly accessible in browsers. Special case: uploading `favicon.ico` makes it the site favicon.

4. **Reserved Paths**: `/` (index page), `/client.js` (JavaScript client), `/vendor/*` assets and `/_admin` (plus everything under `/_admin/`) are special-cased in the handler and cannot be used as storage keys.

## Important Notes

- **Tests**: HTTP handler tests live in `http_test.go` (run with `go test ./...`). `-race` needs `-gcflags=all=-d=checkptr=0` because boltdb v1.3.1 trips checkptr
- **Single binary**: Everything compiles into one executable
- **No TLS**: Cubby does not provide HTTPS - use a reverse proxy (NGINX/Caddy) for production
- **Gob encoding**: User and metadata structs use Go's gob encoding, which matches fields by name, ignores stored fields the struct lacks, and zero-fills fields the record lacks. Field names must remain consistent, and permission fields must be chosen so their zero value is the least privileged (`Admin: false`, `Public: false`), so records missing them fail closed
