# AGENTS.md

This file provides guidance to coding agents when working with code in this
repository.

## Commands

```sh
go run .          # Run the server (listens on localhost:8080 by default)
go build ./...    # Build
go test ./...     # Run all tests
go test ./internal/db/...  # Run DB tests only
```

## Architecture

Hest is a server-side rendered Go web app for tracking game results on a leaderboard. It uses Go's `html/template`, HTMX for partial updates, SQLite (`go-sqlite3`) for persistence, and Basic Auth for write operations.

**Scoring:** 3 points per win, 1 per 2nd place. Tiebreakers in order: wins → seconds → games played → name.

### Package structure

- `main` (root package) — HTTP handlers, app logic, template rendering
- `internal/db` — All database access via `Store` struct; schema creation and incremental migrations in `db.go`
- `internal/weather` — Fetches weather emoji+temp from Open-Meteo historic API at game save time

### Key files

- `main.go` — Entry point; opens DB, runs weather backfill on startup, starts server
- `app.go` — `App` struct (wraps `Store`), route registration, shared helpers (`parseIDs`, `validatePlacement`, etc.)
- `auth.go` — `requireAuth` / `ensureAuthAndForm` for write-protected endpoints
- `assets.go` — Embeds `templates/*.html` and `static/*` into the binary
- `template.go` — `renderTemplate`/`render` helpers; template funcs: `add`, `subtract`, `version`, `hasEasterEgg`
- `season.go` — Season selection logic (auto-selects active season, falls back to latest)
- `leaderboard.go` — Leaderboard handler with rank-change calculation (compares last 2 games)
- `rank.go` — `calculateRankChanges` implementation

### Data model

The DB has three main tables: `players`, `games`, and `game_players` (many-to-many join). A `seasons` table defines named date ranges; DB triggers prevent overlapping seasons. Schema migrations are applied via `migrate()` in `db.go` using `pragma_table_info` checks.

### HTMX pattern

Handlers detect `HX-Request: true` headers and return only the relevant partial template block (e.g. `"content"` or `"score"`) instead of the full `"layout"`. Write handlers also detect `HX-Redirect` for client-side redirects.
