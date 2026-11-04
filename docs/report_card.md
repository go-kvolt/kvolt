# Local quality checks (Go Report Card style)

[Go Report Card](https://goreportcard.com/) **was sunset** (public site/badge no longer grades repos). The old README badge is removed; quality signal is **CI** (`gofmt`, `go vet`, `govulncheck`, tests) plus these local checks.

The official successor they recommend is [golangci-lint](https://golangci-lint.run/).

## Quick (built-in Go tools)

From the repo root:

```bash
go fmt ./...
go vet ./...
go test ./...
```

## Using the Makefile

```bash
make report
```

This runs `fmt`, `vet`, `test`, and (if installed) **ineffassign**, **misspell**, and **staticcheck**. Optional tools are skipped with a message if not installed.

For only format + vet + optional linters (no test):

```bash
make lint
```

## Optional tools

| Check | Install |
| :--- | :--- |
| ineffassign | `go install github.com/gordonklaus/ineffassign@latest` |
| misspell | `go install github.com/client9/misspell/cmd/misspell@latest` |
| staticcheck | `go install honnef.co/go/tools/cmd/staticcheck@latest` |

```bash
ineffassign ./...
misspell -error .
staticcheck ./...
```

Or **golangci-lint**:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run ./...
```

## Optional: goreportcard-cli (self-run)

The CLI from the old project still exists if you want the classic checklist locally:

```bash
go install github.com/gojp/goreportcard/cmd/goreportcard-cli@latest
goreportcard-cli
```

---

**Summary:** Prefer CI + `make report` / `golangci-lint`. Do not rely on goreportcard.com badges — the service is retired.
