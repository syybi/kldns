param(
    [string]$ConfigPath = "configs/config.yaml"
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot

Push-Location $projectRoot
try {
    $env:KLDNS_CONFIG = $ConfigPath
    go run ./cmd/server
} finally {
    Pop-Location
}
