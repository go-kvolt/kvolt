# Reproducible KVolt microbenchmarks (CPU-bound, in-process ServeHTTP / router / JSON).
# Not a network load test — use wrk/hey against a real server for end-to-end Req/Sec.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$env:GOTOOLCHAIN = "go1.27.3"

Write-Host "== go version =="
go version

Write-Host "== hardware =="
Write-Host "GOMAXPROCS=$([Environment]::ProcessorCount)"

Write-Host "== engine (ServeHTTP) =="
go test -run=^$ -bench=BenchmarkEngine_ -benchmem -count=3 -benchtime=1s .

Write-Host "== router =="
go test -run=^$ -bench=BenchmarkRouter_ -benchmem -count=3 -benchtime=1s ./router

Write-Host "== context JSON =="
go test -run=^$ -bench='BenchmarkBindJSON|BenchmarkJSONEncode' -benchmem -count=3 -benchtime=1s ./context

Write-Host "== cache =="
go test -run=^$ -bench=BenchmarkCache -benchmem -count=3 -benchtime=1s ./pkg/cache

Write-Host "Done. Prefer the median of 3 runs. Convert ns/op to ops/sec: 1e9 / ns_per_op."
