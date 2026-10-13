# PK-status

Live status checker for Politechnika Krakowska (Cracow University of Technology)
services. A Go backend periodically probes each target, measuring HTTP latency and
inspecting the TLS certificate, and stores every result in SQLite. An Astro + React +
Tailwind frontend shows a live status card and a latency history chart.

## Stack

- **Backend** — Go, stdlib `net/http`, `modernc.org/sqlite` (pure Go, no CGO).
- **Frontend** — Astro + React islands + Tailwind v4, charts via recharts.
- **Delivery** — Docker (Alpine images), orchestrated with docker-compose. nginx
  serves the static frontend and reverse-proxies `/api/*` to the backend.

## Run

```bash
docker compose up --build
```

Then open http://localhost:8080.

## Configuration (backend env vars)

| Variable        | Default                 | Description                             |
| --------------- | ----------------------- | --------------------------------------- |
| `TARGETS`       | `https://pk.edu.pl`     | Comma-separated list of URLs to check.  |
| `POLL_INTERVAL` | `60s`                   | How often each target is checked.       |
| `DB_PATH`       | `/data/pk-status.db`    | SQLite file path (mounted volume).      |
| `PORT`          | `8080`                  | Backend HTTP port.                      |

Add more checkpoints later by extending `TARGETS` in `docker-compose.yml`.

## API

- `GET /api/health` — liveness.
- `GET /api/status` — latest result per target.
- `GET /api/history?target=<url>&limit=<n>` — recent results (oldest first).

## Local development

```bash
# Backend
cd backend && go run .            # serves on :8080, writes ./pk-status.db by default via env

# Frontend
cd frontend && npm install && npm run dev
```
