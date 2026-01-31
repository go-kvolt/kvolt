# KVolt CLI 🛠️

The KVolt Command Line Interface (CLI) is a powerful tool designed to speed up your development workflow.

## Installation

```bash
go install github.com/go-kvolt/kvolt/cmd/kvolt@latest
```

## Commands

### `kvolt new <project_name>`

Creates a new KVolt project with a recommended directory structure.

**Usage:**

```bash
kvolt new my-awesome-api
```

**What it does:**
1.  Creates project directories (`cmd`, `internal`, `pkg`).
2.  Initializes `go.mod`.
3.  Creates default configuration files (`.env`, `config.yaml`).
4.  Generates a starter `main.go` with example code.

### `kvolt run`

Starts the application in **Development Mode**.

**Usage:**

```bash
cd my-project
kvolt run
```

**Features:**
-   **Hot Reload**: Automatically watches for file changes (`.go`, `.yaml`, `.env`, `.html`) and restarts the server.
-   **Output**: Pipes `stdout` and `stderr` directly to your terminal.
-   **Debounce**: Intelligent debouncing ensures the server doesn't restart multiple times for a single save operation.

> **Note:** `kvolt run` expects your entry point to be at `cmd/api/main.go` or `main.go`.
