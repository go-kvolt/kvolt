# Benchmarks

KVolt publishes **reproducible Go microbenchmarks**, not marketing Req/Sec claims.

## What we measure

| Suite | Path | What it covers |
| :--- | :--- | :--- |
| Engine | `bench_test.go` | In-process `ServeHTTP` (ping, JSON, params, BindJSON, `Default()` middleware) |
| Router | `router/bench_test.go` | Radix `Find` (static / param / 5-param / catch-all) |
| Context | `context/bench_test.go` | `BindJSON` + `JSON` encode |
| Cache | `pkg/cache` | Memory store Set/Get |

These are **CPU microbenchmarks** (no real TCP, no client, discard writer). They are useful for regressions and comparing changes on the same machine.

They are **not**:

- End-to-end HTTP Req/Sec (use `wrk`, `hey`, or `vegeta` against `app.Run`)
- Cross-framework leaderboards (different benches ≠ fair comparison)

## How to run

```powershell
./scripts/bench.ps1
```

Or individually:

```bash
go test -run=^$ -bench=BenchmarkEngine_ -benchmem -count=3 .
go test -run=^$ -bench=BenchmarkRouter_ -benchmem -count=3 ./router
go test -run=^$ -bench='BenchmarkBindJSON|BenchmarkJSONEncode' -benchmem -count=3 ./context
```

Throughput from a serial bench: **ops/sec ≈ 1e9 / ns_per_op**.

## Reporting results

Always include:

1. `go version` / toolchain
2. CPU model and `GOMAXPROCS`
3. OS/arch
4. Exact `go test -bench=…` command
5. Median of ≥3 runs (`-count=3`)

Do **not** paste a single absolute Req/Sec number into docs without that context.
