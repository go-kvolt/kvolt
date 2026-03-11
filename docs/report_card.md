# Running Go Report Card Checks Locally

[Go Report Card](https://goreportcard.com/report/github.com/go-kvolt/kvolt) runs several checks on the codebase. You can run the same style of checks on your machine before pushing.

## Quick (built-in Go tools)

From the **kvolt** directory:

```bash
cd kvolt
go fmt ./...
go vet ./...
go test ./...
```

- **go fmt** — formatting (must be clean for Report Card).
- **go vet** — static analysis.
- **go test** — unit tests.

## Using the Makefile

From the **kvolt** directory:

```bash
make report
```

This runs `fmt`, `vet`, `test`, and (if installed) **ineffassign**, **misspell**, and **staticcheck**. Optional tools are skipped with a message if not installed.

For only format + vet + optional linters (no test):

```bash
make lint
```

## Optional tools (for full parity with Report Card)

Install these to match what goreportcard.com runs:

| Check        | Install |
|-------------|---------|
| ineffassign | `go install github.com/gordonklaus/ineffassign@latest` |
| misspell    | `go install github.com/client9/misspell/cmd/misspell@latest` |
| staticcheck | `go install honnef.co/go/tools/cmd/staticcheck@latest` |

Then run:

```bash
ineffassign ./...
misspell -error .
staticcheck ./...
```

Or use **golangci-lint** (single tool, many linters):

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
```

## One-shot: goreportcard-cli

To run the same CLI that the Report Card site uses:

```bash
go install github.com/gojp/goreportcard/cmd/goreportcard-cli@latest
cd kvolt
goreportcard-cli
```

Verbose output:

```bash
goreportcard-cli -v
```

---

**Summary:** Use `make report` (or `go fmt ./... && go vet ./... && go test ./...`) for a fast local check. Add the optional tools or `goreportcard-cli` when you want to mirror the full Report Card.
