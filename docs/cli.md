# KVolt CLI 🛠️

The KVolt Command Line Interface (CLI) speeds up your development workflow.

## Installation

```bash
go install github.com/go-kvolt/kvolt/cmd/kvolt@latest
```

## Commands

### `kvolt new <project_name>`

Creates a new KVolt project with a recommended directory structure.

```bash
kvolt new my-awesome-api
kvolt new -h   # show help for new
```

**What it does:**
1.  Creates project directories (`cmd/api`, `internal`, `pkg`).
2.  Initializes `go.mod`.
3.  Creates default configuration files (`.env`, `config.yaml`).
4.  Generates a starter `main.go` with example code.

### `kvolt run [-e entry]`

Starts the application in **development mode** with hot reload.

```bash
kvolt run
kvolt run -e cmd/server/main.go
kvolt run -h   # show help for run
```

| Flag | Description |
|------|-------------|
| `-e` | Entry point path (default: `cmd/api/main.go`, then `main.go`) |

**Features:**
-   **Hot reload**: Watches `.go`, `.yaml`, `.env`, `.html` and restarts the server. Skips `.git`, `vendor`, `node_modules`.
-   **Debounce**: Restarts at most once per 500ms to avoid multiple restarts on a single save.

### `kvolt build [-o output] [-e entry]`

Builds the application to a binary.

```bash
kvolt build
kvolt build -o bin/myapp
kvolt build -e cmd/server/main.go
kvolt build -h
```

| Flag | Description |
|------|-------------|
| `-o` | Output path (default: `bin/app`) |
| `-e` | Entry point (default: `cmd/api/main.go`, then `main.go`) |

### `kvolt test [-cover]`

Runs tests.

```bash
kvolt test
kvolt test -cover
kvolt test -h
```

| Flag | Description |
|------|-------------|
| `-cover` | Enable coverage |

### `kvolt fmt`

Formats code with `go fmt ./...`.

```bash
kvolt fmt
```

### `kvolt key`

Prints a random 32-byte hex secret (for JWT, session keys, etc.).

```bash
kvolt key
```

### `kvolt generate handler <name>`

Creates a handler stub in `internal/handler/<name>.go`.

```bash
kvolt generate handler users
# Creates internal/handler/users.go
```

### `kvolt generate middleware <name>`

Creates a middleware stub in `internal/middleware/<name>.go`.

```bash
kvolt generate middleware auth
# Creates internal/middleware/auth.go
```

### `kvolt docker`

Generates a multi-stage `Dockerfile` in the current directory (builds `./cmd/api`, runs on Alpine).

```bash
kvolt docker
```

### `kvolt version`

Prints the CLI version.

```bash
kvolt version
kvolt -v
kvolt --version
```

### Global flags

| Flag | Description |
|------|-------------|
| `-h`, `--help` | Show usage help |
| `-v`, `--version` | Show version |
