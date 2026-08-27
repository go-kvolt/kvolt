$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot\..

$gcc = Get-Command gcc -ErrorAction SilentlyContinue
if (-not $gcc) {
    $wingetGcc = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
    if (Test-Path (Join-Path $wingetGcc "gcc.exe")) {
        $env:Path = "$wingetGcc;$env:Path"
        $gcc = Get-Command gcc
    }
}
if (-not $gcc) {
    throw "gcc is required for local 10/10 (Go race detector). Install WinLibs MinGW and re-run."
}

$env:CGO_ENABLED = "1"

Write-Host "== gcc $($gcc.Source) =="
Write-Host "== gofmt =="
$fmt = gofmt -l .
if ($fmt) {
    Write-Host $fmt
    throw "gofmt needed"
}

Write-Host "== go vet =="
go vet ./...

Write-Host "== go test -race =="
go test -race -count=1 -timeout 10m ./...

Write-Host "== soak 2m =="
$env:KVOLT_SOAK = "1"
$env:KVOLT_SOAK_SEC = "120"
go test -count=1 -timeout 5m -run TestSoak .
Remove-Item Env:KVOLT_SOAK
Remove-Item Env:KVOLT_SOAK_SEC

Write-Host "== fuzz =="
go test -fuzz=FuzzBindJSON -fuzztime=20s ./context
go test -fuzz=FuzzQuery -fuzztime=10s ./context
go test -fuzz=FuzzRouterFind -fuzztime=20s ./router

Write-Host "== install CLI =="
go install ./cmd/kvolt
kvolt version

Write-Host "== module path =="
$mod = go list -m
if ($mod -notmatch '/v2$') {
    throw "module path must end with /v2 (got $mod)"
}

Write-Host "LOCAL 10/10 passed."
