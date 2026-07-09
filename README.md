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
internal/
  solver/       hand-written scheduler: greedy + simulated annealing,
                hard constraints checked at assignment time, pluggable
                soft-constraint scoring, 2–3 candidates for voting
  llm/          provider-agnostic LLM client: availability parsing,
                rule translation, announcement formatting
  telegram/     bot handlers: availability intake, reminders, voting,
                owner approval/veto
  store/        Postgres repositories (pgx, plain SQL); shop_id on
                everything
  scheduler/    cron runner: weekly reminder, nag non-responders,
                close voting & finalize
  config/       env-based configuration
web/            frontend placeholder, embedded into cmd/server via go:embed
migrations/     numbered plain-SQL migrations
```

Dependency rules: `cmd -> internal`; `solver` imports no other shiftbot
package; `store` never imports `llm`; `telegram` may use `llm` and `store`.

## Run locally

Requires Go 1.24+ and Docker.

```sh
# 1. Start Postgres
docker compose up -d db

# 2. Apply migrations (plain SQL, numbered — apply in order)
export DATABASE_URL='postgres://shiftbot:shiftbot@localhost:5432/shiftbot?sslmode=disable'
psql "$DATABASE_URL" -f migrations/0001_init.sql

# 3. Build everything / run tests
go build ./...
go test ./...

# 4. Run the API + dashboard (http://localhost:8080)
go run ./cmd/server

# 5. Run the Telegram bot (webhook mode)
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

Initial project structure. The solver is real working code; the other
packages are compiling skeletons: interface definitions, one end-to-end
example bot handler (availability intake), one example repository per core
table, and a placeholder dashboard wired for `go:embed`. Concrete LLM
providers plug into `internal/llm.Provider` and are selected in
`cmd/bot/main.go`.
