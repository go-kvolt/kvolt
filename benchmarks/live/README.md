# Live HTTP framework comparison

End-to-end localhost load test (real TCP + JSON `/ping`) for:

- **kvolt** (`net/http`)
- **bunrouter** (`net/http` router)
- **gin** (`net/http`)
- **echo** (`net/http`)
- **chi** (`net/http`)
- **stdlib** (`net/http` ServeMux)
- **fiber** (`fasthttp` — different engine; expect higher RPS)

Published sample numbers (with laptop/CPU): [docs/benchmarks.md](../../docs/benchmarks.md#live-http-comparison).

## Requirements

- Go 1.27+
- [bombardier](https://github.com/codesenberg/bombardier): `go install github.com/codesenberg/bombardier@latest`

## Run

```powershell
cd benchmarks/live
./run.ps1      # one pass → results.json
./run10.ps1    # 10 separate rounds → results-10x.json (preferred for docs)
```

Env overrides: `BENCH_C` (connections, default 125), `BENCH_D` (duration, default `10s`), `BENCH_WARMUP` (default `2s`).

## Sample machine (2026-11-04)

| | |
| :--- | :--- |
| Laptop | Lenovo Yoga Slim 7 14IMH9 (83CV) |
| CPU | Intel Core Ultra 7 155H (16C / 22T) |
| OS / Go | Windows 11 amd64 · go1.27.3 |
| Load | bombardier · 125 conn · 10s · 2s warmup · median of 10 rounds |

| Framework | Engine | Median RPS |
| :--- | :--- | ---: |
| Fiber | fasthttp | 87356 |
| KVolt | net/http | 81052 |
| Gin | net/http | 70006 |
| stdlib | net/http | 67881 |
| Echo | net/http | 67367 |
| Chi | net/http | 65854 |

## Fairness notes

- Same machine, sequential runs (one server at a time)
- Same handler shape: `GET /ping` → `{"message":"pong"}`
- Keep-alive enabled (bombardier default)
- Fiber is not an apples-to-apples `net/http` compare; include it as a fasthttp reference only
- Numbers vary by OS, CPU load, and power plan — re-run locally
