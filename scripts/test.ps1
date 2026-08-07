$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot

Push-Location $projectRoot
try {
    go test ./...
    npm --prefix web run typecheck
    npm --prefix web run lint
} finally {
    Pop-Location
}
