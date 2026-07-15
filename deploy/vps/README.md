# VPS Deployment

shiftZ uses the same deployment pattern as Roomio: clone the repository once,
keep `.env` on the VPS, then let GitHub Actions SSH in, pull `main`, build, run
migrations, and restart Docker Compose.

Cloudflare Tunnel routes the public hostname to `http://127.0.0.1:8088`. No
Nginx is required.

## First VPS Setup

```sh
cd /home/ubuntu
git clone https://github.com/betallsoph/shiftZ.git
cd shiftZ

cp .env.example .env
nano .env
chmod 600 .env

docker compose up -d --build
```

Use the Neon pooled URL for `DATABASE_URL` and the direct URL for
`MIGRATION_DATABASE_URL`.

## GitHub Actions

Create a GitHub Environment named `production` with four secrets:

| Secret | Value |
| --- | --- |
| `DEPLOY_HOST` | VPS IP or hostname |
| `DEPLOY_USER` | `ubuntu` |
| `DEPLOY_SSH_KEY` | Contents of the private key used to SSH into the VPS |
| `DEPLOY_PATH` | `/home/ubuntu/shiftZ` |

Every push to `main`, or a manual `Deploy VPS` workflow run, performs:

1. `git fetch` and `git reset --hard origin/main`.
2. Run `docker compose up -d --build`.
3. Compose builds ARM64, applies Atlas migrations, then starts the app.
4. Verify `/livez` plus `/readyz`.

No GHCR account, PAT, repository variable, migration secret, or `/opt` folder
is required.

## Cloudflare Tunnel

Insert this hostname before the existing final catch-all rule:

```yaml
ingress:
  - hostname: shiftz.example.com
    service: http://127.0.0.1:8088
  - service: http_status:404
```

Then reload `cloudflared` and configure Telegram:

```sh
cd /home/ubuntu/shiftZ
set -a
source .env
set +a

curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://shiftz.example.com/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

## Manual Operations

```sh
cd /home/ubuntu/shiftZ

git pull
docker compose up -d --build

docker compose ps
docker compose logs -f --tail=100 app
```

The app is capped at `0.70` CPU and `768m` memory to leave capacity for Roomio.
If `cloudflared` runs in Docker instead of directly on the host, attach it to
`shiftz-prod_default` and route to `http://app:8088`.
