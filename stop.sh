#!/usr/bin/env bash
# stop.sh — Shut down the ThereExists local dev environment (Linux/macOS).
# Usage: ./stop.sh
# Reads .dev-state.json written by dev.sh and tears down:
#   - Go server process tree
#   - Vite client process tree
#   - the project-local Postgres in .pgdata/
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$root"

pgdata="$root/.pgdata"
state_file="$root/.dev-state.json"

# Kill a process and its descendants (TERM, then KILL).
stop_tree() {
    local pid="$1" label="$2"
    [ -n "$pid" ] || return 0
    if ! kill -0 "$pid" 2>/dev/null; then
        echo "  $label (PID $pid) already gone."
        return 0
    fi
    echo "  Killing $label (PID $pid) and children..."
    pkill -TERM -P "$pid" 2>/dev/null || true
    kill -TERM "$pid" 2>/dev/null || true
    sleep 1
    pkill -KILL -P "$pid" 2>/dev/null || true
    kill -KILL "$pid" 2>/dev/null || true
}

# Fallback: kill whatever is listening on a port (uses ss; lsof not present here).
stop_port() {
    local port="$1" label="$2"
    local pids
    pids="$(ss -tlnpH "sport = :$port" 2>/dev/null \
        | grep -o 'pid=[0-9]*' | cut -d= -f2 | sort -u || true)"
    [ -n "$pids" ] || return 0
    for pid in $pids; do
        stop_tree "$pid" "$label"
    done
}

# ── Go server + Vite client ──────────────────────────────────────────────────
if [ -f "$state_file" ]; then
    server_pid="$(grep -o '"server":[0-9]*' "$state_file" | cut -d: -f2 || true)"
    client_pid="$(grep -o '"client":[0-9]*' "$state_file" | cut -d: -f2 || true)"
    pgweb_pid="$(grep -o '"pgweb":[0-9]*' "$state_file" | cut -d: -f2 || true)"
    stop_tree "${server_pid:-}" "Go server"
    stop_tree "${client_pid:-}" "Vite client"
    stop_tree "${pgweb_pid:-}" "pgweb"
else
    echo "No .dev-state.json found - falling back to port-based kill."
    stop_port 8080 "Go server"
    stop_port 5173 "Vite client"
    stop_port 8081 "pgweb"
fi

# ── Project-local Postgres ────────────────────────────────────────────────────
if command -v pg_ctl >/dev/null 2>&1 && pg_ctl -D "$pgdata" status >/dev/null 2>&1; then
    echo "Stopping Postgres..."
    pg_ctl -D "$pgdata" stop -m fast >/dev/null
else
    echo "Postgres not running."
fi

# ── Clear state ────────────────────────────────────────────────────────────────
rm -f "$state_file"

echo ""
echo "All ThereExists dev services stopped."
echo "(Database data is preserved in .pgdata/ — delete that folder to reset.)"
