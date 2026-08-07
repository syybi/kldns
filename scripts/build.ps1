$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $projectRoot "bin"
$windowsBinary = Join-Path $binDir "kldns-windows-amd64.exe"
$linuxBinary = Join-Path $binDir "kldns-linux-amd64"

Push-Location $projectRoot
try {
    npm --prefix web run build
    if ($LASTEXITCODE -ne 0) {
        throw "frontend build failed with exit code $LASTEXITCODE"
    }

    New-Item -ItemType Directory -Force -Path $binDir | Out-Null

    $previousGOOS = $env:GOOS
    $previousGOARCH = $env:GOARCH
    $previousCGOEnabled = $env:CGO_ENABLED
    try {
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        go build -trimpath -ldflags="-s -w" -o $windowsBinary ./cmd/server
        if ($LASTEXITCODE -ne 0) {
            throw "Windows amd64 build failed with exit code $LASTEXITCODE"
        }

        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        $env:CGO_ENABLED = "0"
        go build -trimpath -ldflags="-s -w" -o $linuxBinary ./cmd/server
        if ($LASTEXITCODE -ne 0) {
            throw "Linux amd64 build failed with exit code $LASTEXITCODE"
        }
    } finally {
        $env:GOOS = $previousGOOS
        $env:GOARCH = $previousGOARCH
        $env:CGO_ENABLED = $previousCGOEnabled
    }

    Write-Host "Windows amd64 binary: $windowsBinary"
    Write-Host "Linux amd64 binary: $linuxBinary"
} finally {
    Pop-Location
}
