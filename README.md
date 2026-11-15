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
| `RAW_RETENTION` | `720h` (30 days)        | How long raw check results are kept (`0` = forever). |
| `HOURLY_RETENTION` | `168h` (7 days)      | After this, hourly aggregates of whole UTC days are rolled up into daily ones (`0` = never). |
| `DAILY_RETENTION` | `8760h` (365 days)    | How long daily aggregates are kept (`0` = forever). |

Add more checkpoints by extending `TARGETS` in `docker-compose.yml`, e.g.
`biblioteka=https://www.biblos.pk.edu.pl,dziekanat=https://ehms.pk.edu.pl`.

## API

- `GET /api/health` — liveness.
- `GET /api/status` — latest result per target.
- `GET /api/history?target=<url>&limit=<n>` — recent raw results (oldest first,
  within `RAW_RETENTION`; `cert_issuer` is only reported by `/api/status`).
- `GET /api/uptime?days=<n>&target=<url>` — uptime and average latency per target
  over the last `n` days (default 30, max 365), computed from hourly and daily
  aggregates (the oldest bucket — hour, or UTC day beyond `HOURLY_RETENTION` — is counted in full).

## Data model (SQLite, schema version 2)

| Table           | Contents                                                        |
| --------------- | --------------------------------------------------------------- |
| `targets`       | Dictionary of monitored URLs (`id`, `url`).                     |
| `latest`        | One row per target with its most recent result — backs `/api/status`. |
| `checks_hourly` | Per-target, per-hour count / ok count / latency sum, min, max for the last `HOURLY_RETENTION`. |
| `checks_daily`  | The same per UTC day, rolled up from `checks_hourly`. `/api/uptime` sums both tables. |
| `checks`        | Raw results, `WITHOUT ROWID`, key `(target_id, checked_at)`, pruned after `RAW_RETENTION`. |

Every insert updates `checks`, `checks_hourly` and `latest` in one transaction;
retention runs at startup and then hourly: it deletes old raw rows, moves whole
days of hourly rows into `checks_daily` (in one transaction, so no check is counted
twice or lost) and deletes old daily rows. A database in the old single-table
layout is migrated automatically on startup (and vacuumed). Load test reproducing
60 services × 30 days of per-minute checks:
`cd backend && PKSTATUS_LOAD=1 go test ./internal/storage -run TestLoad -v -timeout 30m`.

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
