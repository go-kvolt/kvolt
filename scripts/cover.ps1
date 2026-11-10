# Library coverage excluding examples/benchmarks (and reporting cmd separately).
$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

$pkgs = go list ./... | Where-Object { $_ -notmatch '/example|/benchmarks' }
go test "-coverprofile=coverage.lib.out" -count=1 @pkgs
go tool cover "-func=coverage.lib.out" | Select-String "total:"

Write-Host ""
Write-Host "cmd/kvolt (separate profile):"
go test "-coverprofile=coverage.cmd.out" -count=1 ./cmd/kvolt/
go tool cover "-func=coverage.cmd.out" | Select-String "total:"
