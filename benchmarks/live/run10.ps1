# 10 separate live comparison rounds; print aggregate (chat/local only).
$ErrorActionPreference = "Continue"
Set-Location $PSScriptRoot
$env:GOTOOLCHAIN = "go1.27.3"

$Connections = 125
$Duration = "5s"
$Warmup = "1s"
$Rounds = 10

$frameworks = @(
    @{ Name = "kvolt";  Port = 18080; Path = "servers/kvolt" },
    @{ Name = "gin";    Port = 18081; Path = "servers/gin" },
    @{ Name = "echo";   Port = 18082; Path = "servers/echo" },
    @{ Name = "chi";    Port = 18083; Path = "servers/chi" },
    @{ Name = "stdlib"; Port = 18084; Path = "servers/stdlib" },
    @{ Name = "fiber";  Port = 18085; Path = "servers/fiber" }
)

function Stop-Tree([int]$ProcId) {
    if ($ProcId -le 0) { return }
    Get-CimInstance Win32_Process -Filter "ParentProcessId=$ProcId" -ErrorAction SilentlyContinue |
        ForEach-Object { Stop-Tree ([int]$_.ProcessId); Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
    Stop-Process -Id $ProcId -Force -ErrorAction SilentlyContinue
}

function Wait-Up([string]$Url, $Proc, [int]$Tries = 60) {
    for ($i = 0; $i -lt $Tries; $i++) {
        Start-Sleep -Milliseconds 250
        try {
            $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 1
            if ($r.StatusCode -eq 200) { return $true }
        } catch {}
        if ($Proc.HasExited) { return $false }
    }
    return $false
}

function Parse-Rps([string]$Out) {
    # Prefer the Statistics "Reqs/sec" Avg column (first number on that line).
    if ($Out -match "Reqs/sec\s+([\d.]+)") {
        return [double]$Matches[1]
    }
    return $null
}

Write-Host "== go mod tidy =="
go mod tidy 2>&1 | Out-Null
Write-Host "go $(go version)  GOMAXPROCS=$([Environment]::ProcessorCount)  rounds=$Rounds  c=$Connections  d=$Duration"

$all = @{}
foreach ($fw in $frameworks) { $all[$fw.Name] = New-Object System.Collections.Generic.List[double] }

for ($r = 1; $r -le $Rounds; $r++) {
    Write-Host ""
    Write-Host "======== ROUND $r / $Rounds ========"
    $roundRows = @()

    foreach ($fw in $frameworks) {
        $addr = "127.0.0.1:$($fw.Port)"
        $url = "http://$addr/ping"
        Write-Host "-- $($fw.Name) $url"

        $env:ADDR = $addr
        $errLog = Join-Path $PSScriptRoot "$($fw.Name).err.log"
        $proc = Start-Process -FilePath "go" -ArgumentList @("run", "./$($fw.Path)") `
            -PassThru -WindowStyle Hidden -RedirectStandardError $errLog -WorkingDirectory $PSScriptRoot
        try {
            if (-not (Wait-Up $url $proc)) {
                Write-Host "FAIL start $($fw.Name)"
                if (Test-Path $errLog) { Get-Content $errLog -ErrorAction SilentlyContinue | Select-Object -Last 5 }
                continue
            }

            & bombardier -c $Connections -d $Warmup --latencies $url 2>&1 | Out-Null
            $out = & bombardier -c $Connections -d $Duration --latencies $url 2>&1 | Out-String
            $rps = Parse-Rps $out
            Write-Host ("   RPS={0}" -f $rps)
            if ($null -ne $rps) {
                [void]$all[$fw.Name].Add($rps)
                $roundRows += [pscustomobject]@{ Framework = $fw.Name; RPS = $rps }
            }
        } finally {
            Stop-Tree $proc.Id
            Start-Sleep -Milliseconds 400
        }
    }

    Write-Host "Round $r summary:"
    $roundRows | Sort-Object RPS -Descending | Format-Table -AutoSize | Out-String | Write-Host
}

function Get-Stats([double[]]$xs) {
    $sorted = @($xs | Sort-Object)
    $n = $sorted.Count
    if ($n -eq 0) {
        return [pscustomobject]@{ Mean = 0; Median = 0; Stdev = 0; Min = 0; Max = 0; N = 0 }
    }
    $mean = ($sorted | Measure-Object -Average).Average
    $med = if ($n % 2 -eq 1) { $sorted[[int]([math]::Floor($n / 2))] } else { ($sorted[$n / 2 - 1] + $sorted[$n / 2]) / 2 }
    $var = 0.0
    foreach ($x in $sorted) { $var += ($x - $mean) * ($x - $mean) }
    $stdev = if ($n -gt 1) { [math]::Sqrt($var / ($n - 1)) } else { 0 }
    [pscustomobject]@{
        Mean   = [math]::Round($mean, 1)
        Median = [math]::Round($med, 1)
        Stdev  = [math]::Round($stdev, 1)
        Min    = [math]::Round($sorted[0], 1)
        Max    = [math]::Round($sorted[$n - 1], 1)
        N      = $n
    }
}

Write-Host ""
Write-Host "======== AGGREGATE ($Rounds separate rounds) ========"
$summary = foreach ($name in @("kvolt", "gin", "echo", "chi", "stdlib", "fiber")) {
    $s = Get-Stats ($all[$name].ToArray())
    [pscustomobject]@{
        Framework = $name
        MedianRPS = $s.Median
        MeanRPS   = $s.Mean
        Stdev     = $s.Stdev
        Min       = $s.Min
        Max       = $s.Max
        N         = $s.N
        Engine    = if ($name -eq "fiber") { "fasthttp" } else { "net/http" }
    }
}
$summary | Sort-Object MedianRPS -Descending | Format-Table -AutoSize
$summary | ConvertTo-Json | Set-Content -Path (Join-Path $PSScriptRoot "results-10x.json") -Encoding utf8
Write-Host "Wrote results-10x.json"
