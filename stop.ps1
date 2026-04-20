# stop.ps1 - Shut down the ThereExists local dev environment.
# Usage: .\stop.ps1
# Reads .dev-state.json written by dev.ps1 and tears down:
#   - Docker containers (docker compose down)
#   - Go server terminal + child go/server process tree
#   - Vite client terminal + child node process tree

$root = $PSScriptRoot
$stateFile = Join-Path $root ".dev-state.json"

function Stop-Tree {
    param($processId, $label)
    if (-not $processId) { return }
    if (-not (Get-Process -Id $processId -ErrorAction SilentlyContinue)) {
        Write-Host "  $label (PID $processId) already gone." -ForegroundColor DarkGray
        return
    }
    Write-Host "  Killing $label (PID $processId) and children..." -ForegroundColor Cyan
    # /T = tree, /F = force. Kills the PowerShell host AND its go/node child.
    & taskkill /PID $processId /T /F 2>&1 | Out-Null
}

$state = $null
if (Test-Path $stateFile) {
    try {
        $state = Get-Content $stateFile -Raw | ConvertFrom-Json
    } catch {
        $state = $null
    }
}

# --- Go server + Vite client terminals ---------------------------------------
if ($state) {
    Stop-Tree -processId $state.server -label "Go server"
    Stop-Tree -processId $state.client -label "Vite client"
} else {
    Write-Host "No .dev-state.json found - falling back to port-based kill." -ForegroundColor Yellow
    $targets = @(
        @{ port = 8080; label = "Go server" },
        @{ port = 5173; label = "Vite client" }
    )
    foreach ($t in $targets) {
        $owners = Get-NetTCPConnection -LocalPort $t.port -State Listen -ErrorAction SilentlyContinue |
                  Select-Object -ExpandProperty OwningProcess -Unique
        foreach ($owningPid in $owners) {
            Stop-Tree -processId $owningPid -label $t.label
        }
    }
}

# --- Docker ------------------------------------------------------------------
Write-Host "Stopping docker compose..." -ForegroundColor Cyan
& docker compose -f "$root\docker-compose.yml" down

# --- Clear state -------------------------------------------------------------
if (Test-Path $stateFile) { Remove-Item $stateFile }

Write-Host ""
Write-Host "All ThereExists dev services stopped." -ForegroundColor Green
