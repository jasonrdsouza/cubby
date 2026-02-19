# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Cubby is a simple, web-native object store written in Go. It provides a REST API for storing and retrieving arbitrary data (JSON, binary files, images, etc.) with basic authentication and authorization. All data is persisted in a single BoltDB file.

## Development Commands

### Building

Build for current platform:
```bash
go fmt && goimports -w . && go vet && go build -o bin/cubby
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

All user commands require direct access to the BoltDB file:

```bash
# Add regular user
./bin/cubby adduser -path data/cubby.db -name username -password password

# Add admin user
./bin/cubby adduser -path data/cubby.db -name username -password password -admin

# List users
./bin/cubby listusers -path data/cubby.db

# Remove user
./bin/cubby removeuser -path data/cubby.db -name username
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
- Sets CORS headers (`Access-Control-Allow-Origin: *`) on all responses and handles OPTIONS preflight
- Serves index page at `/` showing all occupied keys
- Serves the embedded JavaScript client at `/client.js`
- All other paths are treated as keys: `/:key`
- Supports GET, POST (for writes), DELETE methods
- Enforces authorization on every operation based on metadata

**Authentication & Authorization (`users.go`, `metadata.go`)**
- HTTP Basic Auth for all authenticated operations
- Three user groups: Admin, User, Public (defined in `users.go:22-26` - DO NOT reorder these constants)
- Passwords are bcrypt-hashed with cost 8
- Two types: `RegularUser` (authenticated) and `AnonymousUser` (unauthenticated)
- Admins can access everything; regular users can read/write non-admin content; public can only read public content
- Authorization is controlled per-object via custom headers:
  - `X-Cubby-Reader`: sets who can read (admin/user/public)
  - `X-Cubby-Writer`: sets who can write (admin/user)

**Metadata (`metadata.go`)**
- Each key has associated metadata: ContentType, UpdatedAt, Readers group, Writers group
- Defaults: public reads, authenticated-user writes
- Metadata is stored separately from data using gob encoding

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
- Main subcommands: serve, listusers, adduser, removeuser, get, put, remove

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
   - Metadata retrieved for the key
   - `User.InGroup(metadata.Readers/Writers)` checks permission
   - Admins always pass group checks (see `users.go:87-94`)

3. **Web-Native**: The server serves content with proper Content-Type headers, making objects directly accessible in browsers. Special case: uploading `favicon.ico` makes it the site favicon.

4. **Reserved Paths**: `/` (index page) and `/client.js` (JavaScript client) are special-cased in the handler and cannot be used as storage keys.

## Important Notes

- **No test files**: This codebase has no automated tests
- **Single binary**: Everything compiles into one executable
- **No TLS**: Cubby does not provide HTTPS - use a reverse proxy (NGINX/Caddy) for production
- **Group enum order**: The `Group` constants in `users.go:22-26` must never be reordered as the enum values are stored in the database
- **Gob encoding**: User and metadata structs use Go's gob encoding, so field names must remain consistent for backward compatibility
