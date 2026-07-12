# shiftbot

Multi-tenant SaaS that auto-schedules staff shifts for small restaurants and
cafes through a Telegram bot. Employees message their availability in plain
language; an LLM parses it into structured slots; a hand-written solver
(greedy assignment + simulated annealing) generates 2–3 schedule candidates;
the team votes via inline buttons and the owner approves or vetoes.

## Layout

```
cmd/
  bot/          Telegram bot entry point (webhook mode) + cron jobs
  server/       REST API entry point; serves the embedded dashboard
  seed/         seeds a demo shop, employees and shifts (dev only)
  migratediff/  regenerates Atlas migrations from the ent schemas
internal/
  solver/       hand-written scheduler: greedy + simulated annealing,
                hard constraints checked at assignment time, pluggable
                soft-constraint scoring, 2–3 candidates for voting
  llm/          provider-agnostic LLM client: availability parsing,
                rule translation, announcement formatting
  telegram/     bot handlers: availability intake, reminders, voting,
                owner approval/veto
  ent/          ent (entgo.io) data layer: schemas in ent/schema/,
                generated client alongside
  store/        repository layer over the ent client; shop_id on
                everything, callers never see ent types
  scheduler/    cron runner: weekly reminder, nag non-responders,
                close voting & finalize
  config/       env-based configuration
web/            frontend assets, embedded into cmd/server via go:embed
migrations/     versioned Atlas migrations (generated, don't hand-edit)
```

Dependency rules: `cmd -> internal`; `solver` imports no other shiftbot
package; `store` never imports `llm`; `telegram` may use `llm` and `store`;
ent types stay behind `internal/store`.

## Run locally

Requires Go 1.24+, Docker, and the [Atlas CLI](https://atlasgo.io/docs)
(`curl -sSf https://atlasgo.sh | sh`).

```sh
# 1. Start Postgres
docker compose up -d db

# 2. Apply migrations with Atlas
export DATABASE_URL='postgres://shiftbot:shiftbot@localhost:5432/shiftbot?sslmode=disable'
atlas migrate apply --dir file://migrations --url "$DATABASE_URL"

# 3. (optional) Seed demo data (1 shop, 5 employees, a week of shifts
#    and availabilities so the solver can run) — prints an invite code
go run ./cmd/seed

# 4. Build everything / run tests
go build ./...
go test ./...

# 5. Run the API + dashboard (http://localhost:8080)
go run ./cmd/server
```

## Dev API

After seeding, copy the shop `id` from the seed logs (or query Postgres). Then:

```sh
# Generate schedule candidates for a week
curl -X POST "http://localhost:8080/api/v1/dev/generate-schedule?shop_id=<SHOP_ID>&week_start=2026-07-13"

# List persisted candidates and assignments
curl "http://localhost:8080/api/v1/schedules?shop_id=<SHOP_ID>&week_start=2026-07-13"

# Approve one candidate (demotes siblings to draft)
curl -X POST "http://localhost:8080/api/v1/schedules/<SCHEDULE_ID>/approve?shop_id=<SHOP_ID>"
```

`week_start` is interpreted in the shop timezone (from seed: `Asia/Ho_Chi_Minh`).
Generating again for the same shop/week returns `409 Conflict`.

## Dashboard

Open `http://localhost:8080` after `go run ./cmd/server`.

1. Paste the shop `id` printed by `go run ./cmd/seed`.
2. Pick a week start (next Monday works well with seeded availability).
3. Click **Generate** to run the planner, or **Load** to view existing candidates.
4. Click **Approve** on the variant you want.

The dashboard uses HTMX and calls the Go planner/store layer directly (no JSON API from the browser).

```sh
# 6. Run the Telegram bot (webhook mode)
export TELEGRAM_BOT_TOKEN='123456:ABC...'          # from @BotFather
export TELEGRAM_WEBHOOK_SECRET='some-random-string'
go run ./cmd/bot
```

For Telegram to reach the bot locally, expose it with a tunnel (e.g.
`ngrok http 8081`) and register the webhook:

```sh
curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://<your-tunnel>/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

## Environment variables

| Variable                  | Required by     | Default | Description                                    |
| ------------------------- | --------------- | ------- | ---------------------------------------------- |
| `DATABASE_URL`            | bot, server     | —       | Postgres DSN                                   |
| `SERVER_ADDR`             | server          | `:8080` | REST API / dashboard listen address            |
| `BOT_ADDR`                | bot             | `:8081` | Webhook listen address                         |
| `TELEGRAM_BOT_TOKEN`      | bot             | —       | Bot token from @BotFather                      |
| `TELEGRAM_WEBHOOK_SECRET` | bot (optional)  | —       | Must match `secret_token` given to setWebhook  |
| `LLM_PROVIDER`            | bot (optional)  | —       | Model backend; empty disables LLM features     |
| `LLM_API_KEY`             | bot (optional)  | —       | API key for the selected provider              |
| `LLM_MODEL`               | bot (optional)  | —       | Model id for the selected provider             |
| `ENT_DEBUG`               | all (optional)  | —       | `1`/`true` logs every generated SQL statement (dev only) |

## Data layer: ent + Atlas

Entities live in `internal/ent/schema/`; the client is generated next to
them. After changing a schema:

```sh
# 1. Regenerate the ent client (also runs on `go generate ./...`)
go generate ./internal/ent

# 2. Regenerate the migration diff. Atlas needs a scratch "dev database"
#    to replay migrations against — never point this at real data.
docker compose up -d db
psql 'postgres://shiftbot:shiftbot@localhost:5432/shiftbot?sslmode=disable' \
  -c 'CREATE DATABASE dev;' 2>/dev/null || true
DEV_DATABASE_URL='postgres://shiftbot:shiftbot@localhost:5432/dev?sslmode=disable' \
  go run ./cmd/migratediff <migration_name>

# 3. Apply to your database
atlas migrate apply --dir file://migrations --url "$DATABASE_URL"
```

Migration files in `migrations/` are generated by the diff step and hashed
in `atlas.sum` — regenerate rather than hand-editing. `internal/store` wraps
the ent client behind plain repository interfaces, so the rest of the code
(and its tests) never touches ent types; pgx is retained solely as the
`database/sql` driver ent runs on.

## Solver

`internal/solver` is fully self-contained and deliberately dependency-free:

- **Hard constraints** — no double-booking across overlapping shifts,
  per-shift capacity, availability, max hours per employee — are enforced by
  `CanAssign` on every move, so schedules are valid by construction.
  `Validate` re-audits schedules from external sources.
- **Soft constraints** are a pluggable score (`Scorer`): preference
  satisfaction, fairness (stddev of hours), coverage shortfall, plus custom
  `PenaltyRule`s (`AvoidPairRule`, `DayOffRule`, or arbitrary `RuleFunc`s —
  the target for owner rules translated by `internal/llm`).
- **Search** is greedy construction (most constrained shift first) followed
  by simulated annealing over add/drop/replace/swap moves.
- `GenerateCandidates` runs the search under 2–3 different weight presets and
  seeds, deduplicating by assignment fingerprint, to produce distinct
  schedule options for the team vote.

Run its tests with `go test ./internal/solver`.

## Status

The solver, store layer, planner, dev JSON API, and HTMX dashboard form a
working vertical slice: seed data, generate A/B/C schedule candidates, list
them in the dashboard, and approve one variant. Generation is atomic
(all candidates persist in one transaction) and duplicate-protected at the
database level (unique index on shop, week, variant).

Still skeleton / not wired: Telegram bot flows beyond availability intake,
LLM providers (plug into `internal/llm.Provider` in `cmd/bot/main.go`), auth,
and manual schedule editing.
