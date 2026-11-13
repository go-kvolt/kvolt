# Live HTTP comparison: KVolt vs Gin/Echo/Chi/stdlib/Fiber
# Requires: bombardier on PATH, Go 1.27+
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$env:GOTOOLCHAIN = "go1.27.3"
$Connections = if ($env:BENCH_C) { [int]$env:BENCH_C } else { 125 }
$Duration = if ($env:BENCH_D) { $env:BENCH_D } else { "10s" }
$Warmup = if ($env:BENCH_WARMUP) { $env:BENCH_WARMUP } else { "2s" }

$frameworks = @(
    @{ Name = "kvolt";  Port = 18080; Path = "servers/kvolt" },
    @{ Name = "gin";    Port = 18081; Path = "servers/gin" },
    @{ Name = "echo";   Port = 18082; Path = "servers/echo" },
    @{ Name = "chi";    Port = 18083; Path = "servers/chi" },
    @{ Name = "stdlib";    Port = 18084; Path = "servers/stdlib" },
    @{ Name = "bunrouter"; Port = 18086; Path = "servers/bunrouter" },
    @{ Name = "fiber";     Port = 18085; Path = "servers/fiber" }
)

Write-Host "== go mod tidy =="
go mod tidy

Write-Host "== hardware =="
go version
Write-Host "GOMAXPROCS=$([Environment]::ProcessorCount)  connections=$Connections  duration=$Duration"

$results = @()

foreach ($fw in $frameworks) {
    $addr = "127.0.0.1:$($fw.Port)"
    $url = "http://$addr/ping"
    Write-Host ""
    Write-Host "== $($fw.Name) on $addr =="

    $env:ADDR = $addr
    $proc = Start-Process -FilePath "go" -ArgumentList @("run", "./$($fw.Path)") -PassThru -WindowStyle Hidden -RedirectStandardError "$PSScriptRoot\$($fw.Name).err.log"
    try {
        $ok = $false
        for ($i = 0; $i -lt 40; $i++) {
            Start-Sleep -Milliseconds 250
            try {
                $r = Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 1
                if ($r.StatusCode -eq 200) { $ok = $true; break }
            } catch {}
            if ($proc.HasExited) { break }
        }
        if (-not $ok) {
            Write-Host "FAILED to start $($fw.Name)"
            if (Test-Path "$PSScriptRoot\$($fw.Name).err.log") {
                Get-Content "$PSScriptRoot\$($fw.Name).err.log" -ErrorAction SilentlyContinue
            }
            continue
        }

        Write-Host "warmup $Warmup..."
        & bombardier -c $Connections -d $Warmup --latencies $url | Out-Null

        Write-Host "benchmark $Duration..."
        $out = & bombardier -c $Connections -d $Duration --latencies -p r $url 2>&1 | Out-String
        Write-Host $out

        $rps = $null
        if ($out -match "Reqs/sec\s+([\d.]+)") { $rps = [double]$Matches[1] }
        $p50 = $null; $p99 = $null
        if ($out -match "Latency\s+([\d.]+)(us|ms|s)") {
            # bombardier prints histogram; also look for Latency line averages
        }
        if ($out -match "50%\s+([\d.]+)(us|ms|s)") {
            $p50 = "$($Matches[1])$($Matches[2])"
        }
        if ($out -match "99%\s+([\d.]+)(us|ms|s)") {
            $p99 = "$($Matches[1])$($Matches[2])"
        }
        # Prefer Done summary Reqs/sec
        if ($out -match "Done!\s+([\d.]+) requests in") { }
        if ($out -match "([\d.]+)\s+reqs/s") { $rps = [double]$Matches[1] }

        $results += [pscustomobject]@{
            Framework = $fw.Name
            RPS       = $rps
            P50       = $p50
            P99       = $p99
            Engine    = if ($fw.Name -eq "fiber") { "fasthttp" } else { "net/http" }
        }
    } finally {
        if ($proc -and -not $proc.HasExited) {
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
            Get-CimInstance Win32_Process -Filter "ParentProcessId=$($proc.Id)" -ErrorAction SilentlyContinue |
                ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
        }
        Start-Sleep -Milliseconds 500
    }
}

Write-Host ""
Write-Host "== SUMMARY (GET /ping JSON, localhost, bombardier -c $Connections -d $Duration) =="
$results | Sort-Object RPS -Descending | Format-Table -AutoSize
$results | ConvertTo-Json | Set-Content -Path "$PSScriptRoot/results.json" -Encoding utf8
Write-Host "Wrote results.json"
