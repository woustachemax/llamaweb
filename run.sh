#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

pids=()
cleanup() {
  for pid in "${pids[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

echo "starting minigpt voice service on :8000"
( cd ml && exec .venv/bin/uvicorn app:app --host 127.0.0.1 --port 8000 ) &
pids+=($!)

echo "starting go backend on :8080"
( cd backend && exec go run . ) &
pids+=($!)

echo "starting vite frontend on :5173"
( cd frontend && exec npm run dev ) &
pids+=($!)

wait
