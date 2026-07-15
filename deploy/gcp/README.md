# GCP e2-micro Deployment

Production shiftZ runs as one static Go binary under `systemd`. GitHub Actions
tests and builds the binary, applies Atlas migrations to Neon, uploads the
artifact over SSH, then performs an atomic restart with automatic health-check
rollback. The VM does not need Docker, Go, Git, Atlas, or a repository clone.

```text
GitHub Actions
  -> test + vet + linux/amd64 build
  -> Atlas migration using Neon direct URL
  -> SCP to the deployment user's home
  -> root-owned deploy script

GCP e2-micro
  -> shiftz.service -> 127.0.0.1:8088
  -> cloudflared.service -> public HTTPS hostname
  -> Neon pooled connection -> PostgreSQL
```

## 1. Cost And Capacity Check

Before creating anything, review the current [Google Cloud Free Tier](https://docs.cloud.google.com/free/docs/free-cloud-features)
and [VPC network pricing](https://cloud.google.com/vpc/network-pricing).

- The Compute Engine allowance covers one non-preemptible `e2-micro` worth of
  monthly hours in `us-west1`, `us-central1`, or `us-east1`.
- It includes up to 30 GB-months of standard persistent disk and only about
  1 GB/month of eligible outbound transfer from North America.
- `e2-micro` is shared CPU: two visible vCPUs totaling 0.25 sustained vCPU and
  1 GB RAM. It can burst briefly, but it is not a full two-core VM.
- Both static and ephemeral external IPv4 addresses attached to a running
  standard VM are currently billed. At USD 0.005/hour, a continuously running
  IPv4 is roughly USD 3.60 for a 30-day month before tax or currency conversion.
- Traffic to Neon, Telegram, Gemini, Cloudflare, package mirrors, and GitHub can
  count as outbound traffic.
- A budget is an alert, not a hard spending cap. Create budget alerts and inspect
  the SKUs in Billing before and after provisioning.

Avoid `pd-balanced`, `pd-ssd`, snapshots, unused static IPs, GPUs, load balancers,
Cloud NAT, and monitoring agents unless their cost is intentional.

## 2. Create The VM

In Google Cloud Console, create a Compute Engine VM with:

| Setting | Value |
| --- | --- |
| Name | `shiftz-beta` |
| Region | `us-west1` |
| Machine | `e2-micro`, standard/non-Spot |
| Image | Ubuntu 24.04 LTS, x86_64/amd64 |
| Boot disk | 30 GB or less, `pd-standard` |
| Public HTTP/HTTPS firewall | Disabled |
| External IPv4 | Ephemeral is simplest; it is still billable while in use |

The app and Cloudflare Tunnel only need outbound network access. Do not open
ports 80, 443, or 8088. Port 22 is needed for the simple GitHub-hosted Actions
deployment in this repository.

GitHub-hosted runner IP ranges are not fixed enough for a small static firewall
allowlist. For the beta setup, allow TCP 22, enforce key-only SSH using the
bootstrap script, and keep the deployment key dedicated to this VM. A later
hardening step can replace public SSH with IAP/OIDC or a self-hosted runner.

## 3. Create The Deployment Key

On the trusted development machine:

```sh
ssh-keygen -t ed25519 -f ~/.ssh/shiftz_github_actions -C shiftz-deploy
```

Do not add a passphrase because GitHub Actions must use the key unattended. Keep
the private key only in GitHub Environment secrets. The public `.pub` file goes
to the VM during bootstrap.

## 4. Bootstrap The VM

From this repository on the development machine, upload only the provisioning
files and public key. Replace the initial SSH username and IP:

```sh
scp -r deploy/gcp <initial-user>@<vm-ip>:/tmp/shiftz-gcp
scp ~/.ssh/shiftz_github_actions.pub <initial-user>@<vm-ip>:/tmp/
ssh <initial-user>@<vm-ip>
```

On the VM:

```sh
sudo DEPLOY_PUBLIC_KEY="$(cat /tmp/shiftz_github_actions.pub)" \
  /tmp/shiftz-gcp/bootstrap.sh
```

The idempotent script:

- installs only `ca-certificates`, `curl`, `file`, `tzdata`, and OpenSSH runtime
  tools;
- creates the non-login app user `shiftz` and SSH user `shiftz-deploy`;
- installs `/opt/shiftz`, `/etc/shiftz/shiftz.env`, the systemd unit, and
  root-owned deploy/rollback scripts;
- grants the deployment user only exact deploy, rollback, restart, and status
  commands through sudo;
- disables password and root SSH login;
- creates a 1 GB swap safety net with low swappiness;
- caps journald disk usage; and
- preserves an existing production env file on repeated runs.

Swap prevents a sudden allocation from immediately killing the process, but it
does not make the VM faster or replace RAM.

Before closing the initial SSH session, verify the deployment login in another
terminal:

```sh
ssh -i ~/.ssh/shiftz_github_actions shiftz-deploy@<vm-ip>
sudo systemctl status shiftz.service
```

The service will be inactive until the env file is filled and the first binary
is deployed.

## 5. Configure Runtime Secrets

On the VM:

```sh
sudoedit /etc/shiftz/shiftz.env
sudo chmod 600 /etc/shiftz/shiftz.env
sudo chown root:root /etc/shiftz/shiftz.env
```

Use `deploy/gcp/shiftz.env.example` as the field list. Important distinctions:

- `DATABASE_URL`: Neon pooled URL, stored only on the VM for runtime.
- `MIGRATION_DATABASE_URL`: Neon direct URL, stored only as a GitHub secret.
- `APP_ADDR`: exactly `127.0.0.1:8088`.
- `TELEGRAM_WEBHOOK_SECRET`: generate with `openssl rand -hex 32` so it uses
  characters accepted by Telegram's webhook API.
- `COOKIE_SECURE`: `true` behind the public HTTPS tunnel.
- `DEV_API_ENABLED`: `false` in production.
- `REMINDER_MODE`: `loop` because this VM is always on.

Do not put `export` before keys. Keep values containing punctuation inside
single quotes. The service has `GOMAXPROCS=1`, `MemoryHigh=600M`, and
`MemoryMax=700M` to behave predictably on the shared 1 GB VM.

## 6. Configure Cloudflare Tunnel

Create a remotely managed tunnel in Cloudflare Zero Trust and add a public
hostname whose service is:

```text
http://127.0.0.1:8088
```

Install the amd64 host package on the VM:

```sh
curl -L \
  https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb \
  -o /tmp/cloudflared.deb
sudo dpkg -i /tmp/cloudflared.deb
rm /tmp/cloudflared.deb
```

In the Cloudflare tunnel setup page, copy its Linux installation token. Avoid
putting the literal token into shell history:

```sh
read -rsp 'Cloudflare tunnel token: ' CF_TUNNEL_TOKEN
echo
sudo cloudflared service install "$CF_TUNNEL_TOKEN"
unset CF_TUNNEL_TOKEN
sudo systemctl status cloudflared --no-pager
```

The token is a production secret. Never commit it or paste it into GitHub logs.
No Nginx and no public app port are required.

## 7. Configure GitHub Environment

In the repository, create an Environment named `production`. Add these secrets:

| Secret | Value |
| --- | --- |
| `DEPLOY_HOST` | VM external IP or SSH hostname |
| `DEPLOY_USER` | `shiftz-deploy` |
| `DEPLOY_SSH_KEY` | Full contents of `~/.ssh/shiftz_github_actions` |
| `DEPLOY_KNOWN_HOSTS` | Verified SSH host-key line for the VM |
| `MIGRATION_DATABASE_URL` | Neon direct/non-pooler URL |

Get the host key from the VM console or an already verified SSH session instead
of blindly trusting `ssh-keyscan`. On the VM:

```sh
printf '%s ' '<vm-ip>'
sudo cat /etc/ssh/ssh_host_ed25519_key.pub
```

Save the resulting single line, for example
`203.0.113.10 ssh-ed25519 AAAA...`, as `DEPLOY_KNOWN_HOSTS`.

The workflow deliberately keeps `StrictHostKeyChecking=yes`. It never clones a
repository on the VM and never sends runtime secrets over SSH.

## 8. First Deployment

Run the `Deploy GCP` workflow manually, or push to `main`. It performs:

1. `go test ./...` and `go vet ./...`.
2. Static `linux/amd64` build of `./cmd/app`.
3. Atlas migration using the direct Neon URL.
4. Artifact upload to the deployment user's home.
5. Architecture validation and atomic installation under `/opt/shiftz`.
6. Service restart plus `/livez` and `/readyz` checks.
7. Automatic binary rollback if either health check fails.

Migrations run before the new binary is activated. If migration succeeds but
deployment fails, the old binary keeps running against the new schema. Every
production migration must therefore be backward-compatible with the previous
binary. Binary rollback does not rollback database migrations.

Inspect the service on the VM:

```sh
sudo systemctl status shiftz.service --no-pager
journalctl -u shiftz.service -n 100 --no-pager
curl -fsS http://127.0.0.1:8088/livez
curl -fsS http://127.0.0.1:8088/readyz
```

Then verify through the public hostname:

```sh
curl -fsS https://<app-domain>/livez
curl -fsS https://<app-domain>/readyz
```

Use `/livez` for frequent uptime checks. `/readyz` pings Neon and can wake a
scaled-to-zero database.

## 9. Register Telegram Webhook

On the VM, load the protected runtime env in a root shell and register the
public HTTPS endpoint:

```sh
sudo bash -c '
  set -a
  source /etc/shiftz/shiftz.env
  set +a
  curl --fail --silent --show-error \
    "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
    -d "url=https://<app-domain>/telegram/webhook" \
    -d "secret_token=${TELEGRAM_WEBHOOK_SECRET}"
'
```

Check Telegram without printing local env values:

```sh
sudo bash -c '
  source /etc/shiftz/shiftz.env
  curl --fail --silent --show-error \
    "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo"
'
```

## 10. Operations And Recovery

Status, logs, and restart:

```sh
sudo systemctl status shiftz.service --no-pager
journalctl -u shiftz.service -f
sudo systemctl restart shiftz.service
```

Manual binary rollback swaps the active and previous artifacts, restarts, and
checks both health endpoints:

```sh
sudo /usr/local/sbin/shiftz-rollback
```

The automatic deployment rollback prints a short service status and the last 60
journal lines into the failed GitHub Actions job. It never prints env contents.

To rotate a runtime secret:

1. Change it at the provider first when applicable.
2. `sudoedit /etc/shiftz/shiftz.env`.
3. `sudo systemctl restart shiftz.service`.
4. Verify both health endpoints and the affected integration.

Rotating `SESSION_SECRET` logs every dashboard session out. Rotating the
Telegram token requires registering the webhook again. Rotate the GitHub deploy
key by adding the new public key first, updating the Environment secret, testing
a deployment, then removing the old key.

## 11. Upgrade Path

When the VM is constrained, stop it, change the machine type, and start it
again. Moving away from `e2-micro` leaves the free allowance. After an upgrade,
reassess `MemoryHigh`, `MemoryMax`, `GOMAXPROCS`, database pool size, and billing
alerts. No application or deployment architecture change is required.

Watch these signals before upgrading:

- repeated OOM or swap activity;
- schedule generation latency;
- sustained CPU throttling;
- service restarts;
- GitHub deployment health-check time; and
- outbound data exceeding the free allowance.
