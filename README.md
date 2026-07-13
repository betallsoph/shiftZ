# shiftbot

Multi-tenant SaaS that auto-schedules staff shifts for small restaurants and
cafes through a Telegram bot. Employees message their availability in plain
language; an LLM parses it into structured slots; a hand-written solver
(greedy assignment + simulated annealing) generates 2–3 schedule candidates;
the team votes via inline buttons and the owner approves or vetoes.

## Layout

```
cmd/
  app/          unified production runtime (dashboard + webhook + reminders)
  bot/          Telegram bot entry point (webhook mode) + cron jobs — local dev
  server/       REST API entry point; serves the embedded dashboard — local dev
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
web/            frontend assets, embedded into cmd/server and cmd/app via go:embed
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

# Or run the unified production binary locally (requires production env vars):
# export DATABASE_URL=... SESSION_SECRET=... TELEGRAM_BOT_TOKEN=... TELEGRAM_WEBHOOK_SECRET=...
# go run ./cmd/app
```

## Dev API

The JSON API is dev-only and disabled by default in production.
The HTMX dashboard does not require `DEV_API_ENABLED`.

Enable it for local curl testing:

```sh
export DEV_API_ENABLED=true
go run ./cmd/server
```

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

### Owner signup (beta)

Owner self-service signup is disabled by default. Enable for beta onboarding:

```sh
export OWNER_SIGNUP_ENABLED=true
go run ./cmd/server
```

Then open `http://localhost:8080/signup` to create a shop. The success page shows (once):

- Shop ID
- Owner dashboard token
- Employee invite code

Optional default shift templates (morning/evening every day) are created when selected.

### Login

1. Sign in at `/login` with the shop `id` and **Owner dashboard token** from signup or `go run ./cmd/seed`.
2. Pick a week start (next Monday works well with seeded availability).
3. Click **Tải lịch** to view existing candidates, or **Tạo lịch** to run the planner.
4. Click **Duyệt** on the variant you want.

The dashboard uses HTMX and calls the Go planner/store layer directly (no JSON API from the browser).
The dashboard also shows weekly availability submission status and parsed slots.

Use the **Nhân viên** panel to view your roster, edit display name / role / weekly hour limits, and suspend or reactivate staff. Suspended employees cannot re-join via `/start` until you reactivate them in the dashboard.

For local dev, `SESSION_SECRET` can be omitted (the server generates an ephemeral secret and logs a warning).
In production, set a long random `SESSION_SECRET` (for example `openssl rand -base64 32`).
Set `COOKIE_SECURE=true` when serving the dashboard over HTTPS.

### Connect Telegram group

After logging in, use the **Telegram group** panel on the dashboard:

1. Click **Tạo mã setup mới** to generate a setup code (shown once; expires in 30 minutes).
2. Add the bot to your shop's Telegram group.
3. In the group, send `/setup tg_setup_...` with the code from the dashboard.
4. The dashboard should show **Đã kết nối** with the group chat ID.

Setup codes are hashed at rest and cleared after a successful connection.

```sh
# 6. Run the Telegram bot (webhook mode) — local dev only; production uses cmd/app
export TELEGRAM_BOT_TOKEN='123456:ABC...'          # from @BotFather
export TELEGRAM_WEBHOOK_SECRET='some-random-string'
go run ./cmd/bot
```

For **production/beta**, run a single service with `go run ./cmd/app` (or the Docker image). Register the webhook on the same domain:

```txt
https://<app-domain>/telegram/webhook
```

For local Telegram bot testing (separate from dashboard), expose the bot with a tunnel:
`ngrok http 8081`) and register the webhook:

```sh
curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://<your-tunnel>/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

### Availability intake

Employees interact with the bot in **private chat** only (DM). Group chats accept `/setup` for owner dashboard connection; other group messages are ignored.

1. Employee joins with `/start <invite-code>` in a private chat with the bot.
2. If an owner has suspended the employee, `/start` will not re-activate them — the owner must use the dashboard **Nhân viên** panel.
3. Employee sends availability in plain language.
4. Bot parses the message (using the shop timezone) and replies with a short summary plus **Confirm** / **Cancel** buttons.
5. Only **Confirm** writes availability to the database; **Cancel** discards the draft.
6. Pending confirmations expire after 30 minutes (stored in the database).

### Availability reminders

Reminders are **disabled** by default. Choose a mode:

| Mode | Use case |
| ---- | -------- |
| `disabled` | Default — no reminder processing |
| `loop` | Always-on host or local dev (`REMINDERS_ENABLED=true` also selects loop) |
| `http` | Cloud Run / scale-to-zero — Cloud Scheduler POSTs to `/internal/reminders/tick` |

**Loop mode** (local dev or always-on):

```sh
export REMINDER_MODE=loop
# or legacy:
export REMINDERS_ENABLED=true
export REMINDER_TICK_INTERVAL=1m
```

**HTTP mode** (production on Cloud Run):

```sh
export REMINDER_MODE=http
export REMINDER_TRIGGER_SECRET='<random-secret>'   # openssl rand -base64 32
```

Cloud Scheduler calls the internal endpoint (keep the secret private):

```sh
curl -i -X POST https://<app-domain>/internal/reminders/tick \
  -H "X-ShiftZ-Reminder-Secret: $REMINDER_TRIGGER_SECRET"
```

Local smoke:

```sh
curl -i -X POST http://localhost:8080/internal/reminders/tick \
  -H "X-ShiftZ-Reminder-Secret: $REMINDER_TRIGGER_SECRET"
```

Each shop uses its own timezone. Default schedule:

- Thursday 10:00 local — weekly availability reminder to all active linked employees
- Saturday 10:00 local — nag only employees who have not submitted for the target week

Delivery rows in the database prevent duplicate sends across worker ticks.

LLM provider setup is not required to boot the bot; without a provider the bot explains that parsing is not configured yet.

To enable Gemini parsing:

```sh
export LLM_PROVIDER=gemini
export LLM_API_KEY='...'
export LLM_MODEL='gemini-3.5-flash'   # optional; swap for cheaper Flash-Lite-style models later
```

## Environment variables

| Variable                  | Required by     | Default | Description                                    |
| ------------------------- | --------------- | ------- | ---------------------------------------------- |
| `DATABASE_URL`            | app, bot, server| —       | Postgres DSN (Neon: pooled URL for runtime)    |
| `MIGRATION_DATABASE_URL`  | migrations only | —       | Direct Postgres DSN for Atlas (Neon: non-pooler)|
| `APP_ADDR`                | app             | —       | Unified app listen address (overrides `PORT`)    |
| `PORT`                    | app (hosted)    | —       | Platform port → `:<PORT>` when `APP_ADDR` unset|
| `SERVER_ADDR`             | server          | `:8080` | REST API / dashboard listen address (dev)      |
| `BOT_ADDR`                | bot             | `:8081` | Webhook listen address (dev)                   |
| `TELEGRAM_BOT_TOKEN`      | app, bot        | —       | Bot token from @BotFather                      |
| `TELEGRAM_WEBHOOK_SECRET` | app, bot        | —       | Must match `secret_token` given to setWebhook  |
| `LLM_PROVIDER`            | app, bot (opt.) | —       | Model backend (`gemini`); empty disables LLM   |
| `LLM_API_KEY`             | app, bot        | —       | Required when `LLM_PROVIDER=gemini` (app prod) |
| `LLM_MODEL`               | app, bot (opt.) | —       | Model id for the selected provider             |
| `REMINDER_MODE`           | app             | `disabled` | `disabled`, `loop`, or `http` (explicit overrides `REMINDERS_ENABLED`) |
| `REMINDER_TRIGGER_SECRET` | app (`http`)    | —       | Auth header for `POST /internal/reminders/tick` |
| `REMINDERS_ENABLED`       | app, bot (legacy)| —      | `true` → `loop` when `REMINDER_MODE` unset     |
| `REMINDER_TICK_INTERVAL`  | app (`loop`), bot | `1m` | How often the in-process loop ticks            |
| `DB_MAX_OPEN_CONNS`       | all             | `5`     | database/sql max open connections              |
| `DB_MAX_IDLE_CONNS`       | all             | `2`     | database/sql max idle connections              |
| `DB_CONN_MAX_LIFETIME`    | all             | `30m`   | Max connection lifetime                        |
| `DB_CONN_MAX_IDLE_TIME`   | all             | `5m`    | Max idle connection time                       |
| `SESSION_SECRET`          | app, server     | —       | HMAC secret for owner dashboard session cookies |
| `COOKIE_SECURE`           | app, server     | `false` | `true` sets Secure on dashboard session cookies |
| `DEV_API_ENABLED`         | app, server     | `false` | Enables unauthenticated dev JSON API          |
| `OWNER_SIGNUP_ENABLED`    | app, server     | `false` | Enables `/signup` owner onboarding flow       |
| `ENT_DEBUG`               | all (optional)  | —       | `1`/`true` logs every generated SQL statement (dev only) |

## Production / beta deployment

Local dev uses Docker Postgres and split binaries (`cmd/server` + `cmd/bot`).
Beta/production runs **one service** via `cmd/app` (or the Docker image below) on a single domain with one Neon pooled connection.

### Binaries

| Command        | Use case |
| -------------- | -------- |
| `cmd/app`      | Production/beta — dashboard, `/telegram/webhook`, reminders |
| `cmd/server`   | Local dev — dashboard only |
| `cmd/bot`      | Local dev — Telegram webhook + reminders only |

```sh
# Production / beta
go run ./cmd/app

# Docker
docker build -t shiftz:local .
docker run --rm -p 8080:8080 --env-file .env shiftz:local
```

Set `COOKIE_SECURE=true` when serving over HTTPS.

Telegram webhook URL on the same host:

```txt
https://<app-domain>/telegram/webhook
```

### Required runtime env (`cmd/app`)

```sh
DATABASE_URL=...              # Neon pooled connection string
TELEGRAM_BOT_TOKEN=...
TELEGRAM_WEBHOOK_SECRET=...
SESSION_SECRET=...            # openssl rand -base64 32
COOKIE_SECURE=true            # HTTPS deployments
LLM_PROVIDER=gemini
LLM_API_KEY=...               # required when LLM_PROVIDER=gemini
LLM_MODEL=gemini-3.5-flash
REMINDER_MODE=http
REMINDER_TRIGGER_SECRET=...   # openssl rand -base64 32
```

Keep `DEV_API_ENABLED` unset or `false` in production.
Keep `OWNER_SIGNUP_ENABLED` unset or `false` in production unless you want open shop creation.

### Optional runtime env

```sh
APP_ADDR=:8080                # or rely on platform PORT
ENT_DEBUG=false
REMINDER_TICK_INTERVAL=1m
DB_MAX_OPEN_CONNS=5
DB_MAX_IDLE_CONNS=2
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m
```

`SERVER_ADDR` and `BOT_ADDR` apply only to the split dev binaries.

### Health checks

`cmd/app` (and the dev binaries) expose:

| Endpoint   | Purpose | Database |
| ---------- | ------- | -------- |
| `GET /livez`   | Liveness probe | No |
| `GET /readyz`  | Readiness probe | Yes (ping) |
| `GET /healthz` | Backward-compatible alias | No (same as `/livez`) |

Use `/livez` for platform liveness probes. Do **not** point frequent liveness checks at `/readyz` — it wakes the database (important on Neon free tier with scale-to-zero).

### Neon notes

- Use the **pooled** connection string for `DATABASE_URL` (app runtime).
- Use the **direct** connection string for `MIGRATION_DATABASE_URL` (Atlas migrations only).
- First request after scale-to-zero can be slower; readiness checks via `/readyz` are fine during deploy verification.
- Keep pool sizes small (`DB_MAX_OPEN_CONNS=5` default) on free/beta tiers.

### CI

GitHub Actions runs `go test ./...` and `go vet ./...` on every push and pull request (see `.github/workflows/ci.yml`).

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

For Neon (or any hosted Postgres with separate pooler/direct endpoints):

```sh
# App runtime — pooled connection
export DATABASE_URL='postgresql://...neon.tech/...?...pooler...'

# Migrations only — direct connection (not used by cmd/server or cmd/bot)
export MIGRATION_DATABASE_URL='postgresql://...neon.tech/...?sslmode=require'
atlas migrate apply --dir file://migrations --url "$MIGRATION_DATABASE_URL"
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

Still skeleton / not wired: schedule generation from Telegram, employee voting
beyond inline callbacks, auth, and manual schedule editing.

Telegram availability uses Gemini when `LLM_PROVIDER=gemini` is set, with
structured JSON parsing, clarification questions for ambiguous input, and a
confirm-before-save flow (in-memory drafts, 30-minute TTL).
