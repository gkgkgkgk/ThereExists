# dev.ps1 — Launch the ThereExists local dev environment
# Usage: .\dev.ps1
# Opens three terminal windows: Docker (Postgres), Go server, Vite client

$root = $PSScriptRoot

# ── 1. Docker (Postgres) ──────────────────────────────────────────────────────
Write-Host "Starting Docker (Postgres)..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command",
    "Set-Location '$root'; docker compose up"

# ── 2. Wait for Postgres to be healthy ───────────────────────────────────────
Write-Host "Waiting for Postgres to be ready..." -ForegroundColor Yellow
$maxWait = 60   # seconds
$elapsed = 0
$ready = $false
while ($elapsed -lt $maxWait) {
    Start-Sleep -Seconds 2
    $elapsed += 2
    $status = & docker compose -f "$root\docker-compose.yml" ps --format json 2>$null |
              ConvertFrom-Json |
              Where-Object { $_.Service -eq "postgres" } |
              Select-Object -ExpandProperty Health -ErrorAction SilentlyContinue
    if ($status -eq "healthy") {
        $ready = $true
        break
    }
    Write-Host "  ...still waiting ($elapsed s)" -ForegroundColor DarkGray
}
if (-not $ready) {
    Write-Host "Postgres did not become healthy in ${maxWait}s. Launching server anyway..." -ForegroundColor Red
}

# ── 3. Go server ─────────────────────────────────────────────────────────────
Write-Host "Starting Go server..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command",
    "Set-Location '$root\server'; `$env:DATABASE_URL='postgres://postgres:password@localhost:5433/thereexists'; `$env:PORT='8080'; `$env:ALLOWED_ORIGINS='http://localhost:5173'; go run ./cmd/..."

# ── 4. Vite client ───────────────────────────────────────────────────────────
Write-Host "Starting Vite client..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command",
    "Set-Location '$root\client'; npm run dev"

Write-Host ""
Write-Host "All services launched:" -ForegroundColor Green
Write-Host "  Postgres  -> localhost:5433"
Write-Host "  API       -> http://localhost:8080"
Write-Host "  Client    -> http://localhost:5173"
