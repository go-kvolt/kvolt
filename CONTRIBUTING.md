# Contributing to KVolt

Thanks for helping improve KVolt.

## Requirements

- Go **1.27+** (see `go.mod` / `toolchain`)
- `gofmt` clean tree before opening a PR

## Development

```bash
go test ./...
go vet ./...
./scripts/bench.ps1          # optional performance check
./scripts/local10.ps1        # fuller local gate (race, soak, fuzz; needs gcc on Windows)
```

## Pull requests

1. Keep changes focused (one concern per PR when possible).
2. Add or update tests for behavior changes.
3. Update docs / `CHANGELOG.md` `[Unreleased]` when the public API changes.
4. Do not invent benchmark marketing numbers — use `scripts/bench.ps1` and document hardware.

## Security

If you believe you found a vulnerability in KVolt itself, please open a private security advisory on GitHub (or email the maintainer) instead of a public issue when disclosure could cause harm.
