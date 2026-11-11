# PK-status

Live status checker for Politechnika Krakowska (Cracow University of Technology)
services. A Go backend periodically probes each target, measuring HTTP latency and
inspecting the TLS certificate, and stores every result in SQLite. An Astro + React +
Tailwind frontend shows a live status card and a latency history chart.

## Stack

- **Backend** — Go, stdlib `net/http`, `modernc.org/sqlite` (pure Go, no CGO).
- **Frontend** — Bun, Astro + React islands + Tailwind v4, charts via recharts.
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
| `TARGETS`       | `https://pk.edu.pl`     | Comma-separated `name=url` entries (bare `url` uses the host as name). |
| `POLL_INTERVAL` | `60s`                   | How often each target is checked.       |
| `DB_PATH`       | `/data/pk-status.db`    | SQLite file path (mounted volume).      |
| `PORT`          | `8080`                  | Backend HTTP port.                      |

Add more checkpoints by extending `TARGETS` in `docker-compose.yml`, e.g.
`biblioteka=https://www.biblos.pk.edu.pl,dziekanat=https://ehms.pk.edu.pl`.

## API

- `GET /api/health` — liveness.
- `GET /api/status` — latest result per target.
- `GET /api/history?target=<url>&limit=<n>` — recent results (oldest first).

## Local development

### With Docker (hot reload)

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

Open http://localhost:8080 as usual. The frontend runs the Astro dev server with
`frontend/` bind-mounted, so edits show up instantly without rebuilding the image.
After changing dependencies, rerun with `--build -V` to refresh `node_modules`.
File watching uses polling (`WATCH_POLLING`) because Docker Desktop's VM doesn't
forward file events from bind mounts.

### Without Docker

```bash
# Backend
cd backend && go run .            # serves on :8080, writes ./pk-status.db by default via env

# Frontend
cd frontend && bun install && bun --bun run dev   # proxies /api to localhost:8080
```
