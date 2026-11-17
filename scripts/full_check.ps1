# Runs every check and saves the results, so nobody has to watch it.
#
#   ./scripts/full_check.ps1                 full run (about 10-15 minutes)
#   ./scripts/full_check.ps1 -Quick          skip fuzzing and shorten the load test
#   ./scripts/full_check.ps1 -LoadSeconds 300
#
# Everything is written to test-results/<timestamp>/ :
#   SUMMARY.txt   one line per step (PASS / FAIL / SKIP) and the overall result
#   <step>.log    full output of each step
# The script keeps going when a step fails and exits non-zero at the end if any did.
param(
    [switch]$Quick,
    [int]$LoadSeconds = 60,
    [string]$Redis = "127.0.0.1:6379",
    [int]$CoverageGate = 90
)

$ErrorActionPreference = "Continue"
Set-Location (Split-Path $PSScriptRoot -Parent)
$root = (Get-Location).Path
if ($Quick -and $PSBoundParameters.ContainsKey("LoadSeconds") -eq $false) { $LoadSeconds = 20 }

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$out = Join-Path $root "test-results\$stamp"
New-Item -ItemType Directory -Force -Path $out | Out-Null
$summary = Join-Path $out "SUMMARY.txt"
$script:results = @()
$started = Get-Date

function Log([string]$msg) {
    Add-Content -Path $summary -Value $msg -Encoding UTF8
    Write-Host $msg
}

# The race detector needs gcc (see scripts/local10.ps1).
if (-not (Get-Command gcc -ErrorAction SilentlyContinue)) {
    $winget = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin"
    if (Test-Path (Join-Path $winget "gcc.exe")) { $env:Path = "$winget;$env:Path" }
}
$env:CGO_ENABLED = "1"
$hasGcc = [bool](Get-Command gcc -ErrorAction SilentlyContinue)

Log "KVolt full check  $stamp"
Log "go: $(go version)"
Log "output: $out"
Log ("-" * 70)

# Run a command line through cmd so stdout and stderr land in one log file, and record the result.
function Step([string]$name, [string]$cmdline, [scriptblock]$check = $null) {
    $log = Join-Path $out ("{0}.log" -f ($name -replace '[^A-Za-z0-9]+', '_'))
    $t = Get-Date
    cmd /c "$cmdline > `"$log`" 2>&1"
    $code = $LASTEXITCODE
    $ok = ($code -eq 0)
    $note = ""
    if ($check) {
        $verdict = & $check $log $code
        if ($verdict) { $ok = $verdict.Ok; $note = $verdict.Note }
    }
    $secs = [int]((Get-Date) - $t).TotalSeconds
    $status = if ($ok) { "PASS" } else { "FAIL" }
    $line = "{0,-5} {1,-34} {2,5}s  {3}" -f $status, $name, $secs, $note
    Log $line
    $script:results += [pscustomobject]@{ Name = $name; Ok = $ok; Skipped = $false }
}

function Skip([string]$name, [string]$why) {
    Log ("{0,-5} {1,-34} {2,5}   {3}" -f "SKIP", $name, "", $why)
    $script:results += [pscustomobject]@{ Name = $name; Ok = $true; Skipped = $true }
}

function Have([string]$exe) { [bool](Get-Command $exe -ErrorAction SilentlyContinue) }

# ---------------------------------------------------------------- static checks
Step "build" "go build ./..."
Step "vet" "go vet ./..."

# gofmt, ignoring CRLF line endings (a Windows checkout may have them).
$fmtLog = Join-Path $out "gofmt.log"
$bad = @()
$tmpGo = Join-Path $out "fmtcheck.go"
$goFiles = @(git ls-files "*.go") + @(git ls-files -o --exclude-standard "*.go")
foreach ($f in $goFiles) {
    if (-not (Test-Path $f)) { continue }
    # Format a copy with LF endings. (Piping text to gofmt from PowerShell adds CRLF back.)
    $text = [System.IO.File]::ReadAllText((Join-Path $root $f)) -replace "`r`n", "`n"
    [System.IO.File]::WriteAllText($tmpGo, $text, (New-Object System.Text.UTF8Encoding($false)))
    $res = gofmt -l $tmpGo 2>&1
    if ($res) { $bad += $f }
}
Remove-Item $tmpGo -ErrorAction SilentlyContinue
$bad | Set-Content $fmtLog
if ($bad.Count -eq 0) { Log ("{0,-5} {1,-34}" -f "PASS", "gofmt (ignoring CRLF)"); $script:results += [pscustomobject]@{ Name = "gofmt"; Ok = $true; Skipped = $false } }
else { Log ("{0,-5} {1,-34} {2} file(s), see gofmt.log" -f "FAIL", "gofmt (ignoring CRLF)", $bad.Count); $script:results += [pscustomobject]@{ Name = "gofmt"; Ok = $false; Skipped = $false } }

if (Have "govulncheck") { Step "govulncheck" "govulncheck ./..." { param($log, $code) @{ Ok = (Select-String -Path $log -Pattern "No vulnerabilities found" -Quiet); Note = "" } } }
else { Skip "govulncheck" "not installed (go install golang.org/x/vuln/cmd/govulncheck@latest)" }

if (Have "gosec") { Step "gosec" "gosec -quiet -exclude-dir=benchmarks -exclude-dir=scripts -fmt=text ./..." }
else { Skip "gosec" "not installed (go install github.com/securego/gosec/v2/cmd/gosec@latest)" }

# ---------------------------------------------------------------- tests
$profile = Join-Path $out "coverage.all.out"
if ($hasGcc) {
    Step "tests (race)" "go test -race -count=1 -timeout 15m -covermode=atomic -coverprofile=`"$profile`" ./..." {
        param($log, $code)
        $races = (Select-String -Path $log -Pattern "WARNING: DATA RACE" -AllMatches | Measure-Object).Count
        $failed = (Select-String -Path $log -Pattern "^(--- FAIL|FAIL\s)" | Measure-Object).Count
        @{ Ok = ($code -eq 0 -and $races -eq 0); Note = "data races: $races, failures: $failed" }
    }
} else {
    Skip "tests (race)" "gcc not found; running without the race detector"
    Step "tests" "go test -count=1 -timeout 15m -covermode=atomic -coverprofile=`"$profile`" ./..."
}

if (Test-Path $profile) {
    $lib = Join-Path $out "coverage.lib.out"
    Get-Content $profile | Where-Object { $_ -notmatch '/example/|/benchmarks/|/cmd/|/scripts/' } | Set-Content $lib
    $line = go tool cover "-func=$lib" | Select-String "^total:"
    $pct = if ($line) { [double](($line.ToString() -split '\s+')[-1].TrimEnd('%')) } else { 0 }
    $ok = $pct -ge $CoverageGate
    Log ("{0,-5} {1,-34} {2}% (gate {3}%)" -f $(if ($ok) { "PASS" } else { "FAIL" }), "library coverage", $pct, $CoverageGate)
    $script:results += [pscustomobject]@{ Name = "coverage"; Ok = $ok; Skipped = $false }
    go tool cover "-html=$lib" "-o=$(Join-Path $out 'coverage.html')" 2>$null
}

# ---------------------------------------------------------------- fuzzing
if ($Quick) { Skip "fuzz" "-Quick" }
else {
    Step "fuzz router" "go test -run xxx -fuzz=FuzzRouterFind -fuzztime=30s ./router"
    Step "fuzz BindJSON" "go test -run xxx -fuzz=FuzzBindJSON -fuzztime=20s ./context"
    Step "fuzz Query" "go test -run xxx -fuzz=FuzzQuery -fuzztime=10s ./context"
}

# ---------------------------------------------------------------- load test and worker kill test
$lt = Join-Path $root "scripts\loadtest"
$bin = Join-Path $out "bin"
New-Item -ItemType Directory -Force -Path $bin | Out-Null
Push-Location $lt
$built = $true
if ($hasGcc) { cmd /c "go build -race -o `"$bin\server.exe`" ./server > `"$out\build_loadtest.log`" 2>&1"; if ($LASTEXITCODE -ne 0) { $built = $false } }
else { cmd /c "go build -o `"$bin\server.exe`" ./server > `"$out\build_loadtest.log`" 2>&1"; if ($LASTEXITCODE -ne 0) { $built = $false } }
if ($hasGcc) { cmd /c "go build -race -o `"$bin\qworker.exe`" ./qworker >> `"$out\build_loadtest.log`" 2>&1"; if ($LASTEXITCODE -ne 0) { $built = $false } }
else { cmd /c "go build -o `"$bin\qworker.exe`" ./qworker >> `"$out\build_loadtest.log`" 2>&1"; if ($LASTEXITCODE -ne 0) { $built = $false } }
cmd /c "go build -o `"$bin\driver.exe`" ./driver >> `"$out\build_loadtest.log`" 2>&1"; if ($LASTEXITCODE -ne 0) { $built = $false }
cmd /c "go build -o `"$bin\qdriver.exe`" ./qdriver >> `"$out\build_loadtest.log`" 2>&1"; if ($LASTEXITCODE -ne 0) { $built = $false }
Pop-Location

if (-not $built) {
    Log ("{0,-5} {1,-34} see build_loadtest.log" -f "FAIL", "build load-test harness")
    $script:results += [pscustomobject]@{ Name = "build load-test"; Ok = $false; Skipped = $false }
} else {
    $passed = { param($log, $code) @{ Ok = ($code -eq 0 -and (Select-String -Path $log -Pattern "RESULT: PASSED" -Quiet)); Note = "" } }

    # Real TCP server (race detector on), mixed traffic, correctness checked on every response,
    # then leak checks and a graceful shutdown with a request in flight.
    Step "load test ($LoadSeconds s)" "`"$bin\driver.exe`" -server `"$bin\server.exe`" -addr 127.0.0.1:18080 -duration ${LoadSeconds}s -workers 64 -log `"$out\load_server.log`"" $passed

    # Kill a worker process while it holds jobs; a second worker must finish every job.
    $redisUp = $false
    try {
        $h, $p = $Redis.Split(":")
        $c = New-Object System.Net.Sockets.TcpClient
        $iar = $c.BeginConnect($h, [int]$p, $null, $null)
        $redisUp = $iar.AsyncWaitHandle.WaitOne(1500) -and $c.Connected
        $c.Close()
    } catch { $redisUp = $false }
    $redisArg = if ($redisUp) { "-redis $Redis" } else { "" }
    $backend = if ($redisUp) { "real Redis $Redis" } else { "in-process Redis emulator (no Redis at $Redis)" }
    Step "queue worker-kill ($backend)" "`"$bin\qdriver.exe`" $redisArg -worker `"$bin\qworker.exe`" -jobs 400" $passed
}

# ---------------------------------------------------------------- summary
$fails = @($script:results | Where-Object { -not $_.Ok })
$skips = @($script:results | Where-Object { $_.Skipped })
$mins = [math]::Round(((Get-Date) - $started).TotalMinutes, 1)
Log ("-" * 70)
if ($fails.Count -eq 0) {
    Log "RESULT: ALL PASSED  ($($script:results.Count - $skips.Count) steps, $($skips.Count) skipped, $mins min)"
} else {
    Log "RESULT: FAILED  ($($fails.Count) step(s): $(($fails | ForEach-Object { $_.Name }) -join ', '))  $mins min"
    Log "Open the matching .log file in $out"
}
# A marker file that is easy to look for.
Set-Content -Path (Join-Path $out ($(if ($fails.Count -eq 0) { "PASSED" } else { "FAILED" }))) -Value $stamp
Set-Content -Path (Join-Path $root "test-results\latest.txt") -Value $out
exit $(if ($fails.Count -eq 0) { 0 } else { 1 })
