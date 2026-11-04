# Benchmarks

KVolt publishes two kinds of numbers:

1. **In-process microbenchmarks** — regressions / same-machine comparisons (`go test -bench`).
2. **Live localhost HTTP** — real TCP + JSON `/ping` vs other frameworks (`benchmarks/live`). Absolute RPS is machine-specific; always re-run locally.

## Microbenchmarks

| Suite | Path | What it covers |
| :--- | :--- | :--- |
| Engine | `bench_test.go` | In-process `ServeHTTP` (ping, JSON, params, BindJSON, `Default()` middleware) |
| Router | `router/bench_test.go` | Radix `Find` (static / param / 5-param / catch-all) |
| Context | `context/bench_test.go` | `BindJSON` + `JSON` encode |
| Cache | `pkg/cache` | Memory store Set/Get |

These are **CPU microbenchmarks** (no real TCP, no client, discard writer).

### How to run

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

Sample numbers: see [README](../README.md#benchmarks).

## Live HTTP comparison

Harness: [`benchmarks/live`](../benchmarks/live) (bombardier → `GET /ping` → `{"message":"pong"}`).

| Setting | Value |
| :--- | :--- |
| Tool | [bombardier](https://github.com/codesenberg/bombardier) |
| Load | 125 connections, 10s duration, 2s warmup |
| Method | 10 separate full rounds; report **median** RPS |
| Engines | kvolt / gin / echo / chi / stdlib = `net/http`; fiber = `fasthttp` (not apples-to-apples) |

### Sample machine

| | |
| :--- | :--- |
| Laptop | Lenovo Yoga Slim 7 14IMH9 (83CV) |
| CPU | Intel Core Ultra 7 155H (16 cores / 22 threads) |
| OS | Windows 11 Home, amd64 |
| Go | go1.27.3 |
| Date | 2026-11-04 |

### Sample results (median RPS over 10 rounds)

| Framework | Engine | Median RPS | Mean | Stdev | Min | Max |
| :--- | :--- | ---: | ---: | ---: | ---: | ---: |
| Fiber | fasthttp | 87356 | 86822 | 4822 | 75399 | 91730 |
| **KVolt** | **net/http** | **81052** | **82526** | **9284** | **65906** | **99434** |
| Gin | net/http | 70006 | 70404 | 6588 | 57804 | 84509 |
| stdlib | net/http | 67881 | 67616 | 5345 | 55545 | 77695 |
| Echo | net/http | 67367 | 68012 | 6344 | 55807 | 81753 |
| Chi | net/http | 65854 | 65719 | 6715 | 51843 | 79971 |

**Reading this:** among `net/http` frameworks on this laptop, KVolt led. Fiber is higher because it uses fasthttp. Stdev is elevated by a few noisy rounds (shared OS load); prefer the median. Your numbers will differ — re-run:

```powershell
cd benchmarks/live
./run10.ps1
```

Single-pass: `./run.ps1`.

## Reporting results

Always include:

1. `go version` / toolchain
2. Laptop / CPU model and `GOMAXPROCS`
3. OS/arch
4. Exact command (`go test -bench=…` or `benchmarks/live/run10.ps1`)
5. Median of ≥3 microbench runs, or ≥10 live rounds when publishing RPS

Do **not** paste a single absolute Req/Sec number into docs without that context.
