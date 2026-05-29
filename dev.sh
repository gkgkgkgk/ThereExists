#!/usr/bin/env bash
# dev.sh — Launch the ThereExists local dev environment (Linux/macOS).
# Usage:  nix develop   (to get the toolchain), then  ./dev.sh
# Starts:
#   - a project-local Postgres in .pgdata/ (no Docker, no daemon, no sudo)
#   - the Go server   (background, logs to .dev-server.log)
#   - the Vite client (background, logs to .dev-client.log)
# Postgres keeps running between dev.sh runs; ./stop.sh shuts everything down.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$root"

pgdata="$root/.pgdata"
pg_log="$root/.dev-postgres.log"
server_log="$root/.dev-server.log"
client_log="$root/.dev-client.log"
state_file="$root/.dev-state.json"

PGPORT=5433
PGUSER=postgres
PGDATABASE=thereexists

# ── 0. Make sure the toolchain is on PATH (i.e. we're inside `nix develop`) ────
missing=""
for c in go node npm pg_ctl initdb pg_isready psql createdb; do
    command -v "$c" >/dev/null 2>&1 || missing="$missing $c"
done
if [ -n "$missing" ]; then
    echo "Missing tools:$missing" >&2
    echo "Enter the dev shell first:  nix develop" >&2
    exit 1
fi

# ── 1. Project-local Postgres ─────────────────────────────────────────────────
if [ ! -d "$pgdata" ]; then
    echo "Initializing Postgres data dir (.pgdata/)..."
    initdb -D "$pgdata" -U "$PGUSER" --auth=trust --encoding=UTF8 --no-locale >/dev/null
fi

if pg_ctl -D "$pgdata" status >/dev/null 2>&1; then
    echo "Postgres already running."
else
    echo "Starting Postgres on localhost:$PGPORT..."
    pg_ctl -D "$pgdata" -l "$pg_log" \
        -o "-p $PGPORT -c listen_addresses=127.0.0.1 -c unix_socket_directories=$pgdata" \
        start
fi

echo "Waiting for Postgres to be ready..."
elapsed=0
until pg_isready -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" >/dev/null 2>&1; do
    sleep 1
    elapsed=$((elapsed + 1))
    if [ "$elapsed" -ge 30 ]; then
        echo "Postgres did not become ready in 30s. See $pg_log" >&2
        exit 1
    fi
done

# Create the application database on first run.
if ! psql -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" -lqt | cut -d'|' -f1 | grep -qw "$PGDATABASE"; then
    echo "Creating database '$PGDATABASE'..."
    createdb -h 127.0.0.1 -p "$PGPORT" -U "$PGUSER" "$PGDATABASE"
fi

# ── 2. Client first-run setup ─────────────────────────────────────────────────
if [ ! -f "$root/client/.env.local" ]; then
    echo "Creating client/.env.local..."
    echo "VITE_API_URL=http://localhost:8080" >"$root/client/.env.local"
fi
if [ ! -d "$root/client/node_modules" ]; then
    echo "Installing client dependencies (npm install)..."
    (cd "$root/client" && npm install)
fi

# ── 3. Go server ──────────────────────────────────────────────────────────────
echo "Starting Go server (logging to .dev-server.log)..."
(
    cd "$root/server"
    DATABASE_URL="postgres://$PGUSER:password@localhost:$PGPORT/$PGDATABASE?sslmode=disable" \
    PORT=8080 \
    ALLOWED_ORIGINS='http://localhost:5173' \
    exec go run ./cmd/...
) >"$server_log" 2>&1 &
server_pid=$!

# ── 4. Vite client ─────────────────────────────────────────────────────────────
echo "Starting Vite client (logging to .dev-client.log)..."
(
    cd "$root/client"
    exec npm run dev
) >"$client_log" 2>&1 &
client_pid=$!

# ── 5. pgweb (database browser UI) ────────────────────────────────────────────
echo "Starting pgweb (logging to .dev-pgweb.log)..."
(
    exec pgweb \
        --url "postgres://$PGUSER:password@localhost:$PGPORT/$PGDATABASE?sslmode=disable" \
        --bind 127.0.0.1 --listen 8081 --skip-open
) >"$root/.dev-pgweb.log" 2>&1 &
pgweb_pid=$!

# ── 6. Record state for stop.sh ──────────────────────────────────────────────
printf '{"server":%d,"client":%d,"pgweb":%d}\n' "$server_pid" "$client_pid" "$pgweb_pid" >"$state_file"

echo ""
echo "All services launched:"
echo "  Postgres  -> localhost:$PGPORT   (.pgdata/, logs: .dev-postgres.log)"
echo "  API       -> http://localhost:8080   (PID $server_pid, logs: .dev-server.log)"
echo "  Swagger   -> http://localhost:8080/swagger/index.html"
echo "  Client    -> http://localhost:5173   (PID $client_pid, logs: .dev-client.log)"
echo "  DB UI     -> http://localhost:8081   (PID $pgweb_pid, logs: .dev-pgweb.log)"
echo ""
echo "Tail logs with:  tail -f .dev-server.log .dev-client.log"
echo "Stop everything: ./stop.sh"
