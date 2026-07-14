# VPS Deployment

shiftZ runs as one ARM64 Docker container. Neon remains external. A host-level
Cloudflare Tunnel routes the public hostname to `http://127.0.0.1:8088`, so no
additional Nginx container is required.

## One-Time VPS Bootstrap

Install Docker Engine with the Compose plugin, then prepare the deploy folder:

```sh
sudo mkdir -p /opt/shiftz
sudo chown "$USER":"$USER" /opt/shiftz
```

Create `/opt/shiftz/.env.production` from `.env.example`, fill the real values,
and lock it down:

```sh
chmod 600 /opt/shiftz/.env.production
```

The SSH deploy user must be able to run Docker without an interactive sudo
prompt. If the GHCR package is private, log in once on the VPS with a GitHub PAT
that has `read:packages`:

```sh
read -rsp 'GHCR PAT: ' GHCR_PAT; echo
printf '%s' "$GHCR_PAT" | docker login ghcr.io -u betallsoph --password-stdin
unset GHCR_PAT
```

## GitHub Production Secrets

Create a GitHub Environment named `production`, then add:

| Secret | Value |
| --- | --- |
| `MIGRATION_DATABASE_URL` | Neon direct, non-pooler URL |
| `VPS_HOST` | VPS hostname or IP |
| `VPS_USER` | SSH deploy user |
| `VPS_PORT` | SSH port; optional, defaults to `22` |
| `VPS_SSH_KEY` | Private deployment key |
| `VPS_KNOWN_HOSTS` | Verified SSH host-key line |

Create the repository variable `VPS_DEPLOY_ENABLED=true` only after all secrets
and `/opt/shiftz/.env.production` are ready. Until then, pushes to `main` skip
the deploy jobs. A manually dispatched workflow always attempts a deployment.

Generate the known-hosts line from a trusted machine and compare its
fingerprint with the VPS console before saving it:

```sh
ssh-keyscan -H -p 22 YOUR_VPS_HOST
```

Pushes to `main` now perform:

1. Build a `linux/arm64` image on GitHub Actions.
2. Push SHA and `latest` tags to GHCR.
3. Apply Atlas migrations to Neon.
4. Upload `compose.prod.yml` and restart the VPS service.
5. Verify `/livez` and `/readyz`; roll back the image if startup fails.

## Cloudflare Tunnel

Add one ingress rule to the existing host-level tunnel:

```yaml
ingress:
  - hostname: shiftz.example.com
    service: http://127.0.0.1:8088
  - service: http_status:404
```

Keep the final catch-all rule already present in your tunnel configuration.
Reload/restart `cloudflared`, then set the Telegram webhook:

```sh
set -a
source /opt/shiftz/.env.production
set +a

curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://shiftz.example.com/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

## Manual Operations

```sh
cd /opt/shiftz

# Status and logs
docker compose -f compose.prod.yml ps
docker compose -f compose.prod.yml logs -f --tail=100 app

# Manual pull/restart using latest
SHIFTZ_IMAGE=ghcr.io/betallsoph/shiftz:latest \
  docker compose -f compose.prod.yml pull app
SHIFTZ_IMAGE=ghcr.io/betallsoph/shiftz:latest \
  docker compose -f compose.prod.yml up -d --no-deps app
```

The Compose service is capped at `0.70` CPU by default to leave CPU time for
Roomio. Override with `SHIFTZ_CPUS` only after observing real VPS metrics. For
a maintenance start without the reminder loop, set
`SHIFTZ_REMINDER_MODE=disabled` on the Compose command.

This Compose file assumes `cloudflared` runs directly on the VPS host. If the
tunnel itself runs in Docker, attach it to the `shiftz-prod_default` network
and route to `http://app:8088`; the loopback published port is not reachable
from a separate container.
