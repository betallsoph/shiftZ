# Manual Production Checklist

Nhung viec chi chu tai khoan GCP, Cloudflare, GitHub, Neon va Telegram moi co
the tu lam. Production hien tai la GCP `e2-micro` + systemd, khong dung Docker
tren VM. Huong dan chi tiet nam tai `deploy/gcp/README.md`.

## 1. Tai Khoan Va Chi Phi

- [ ] Kiem tra Compute Engine Free Tier hien tai truoc khi tao VM.
- [ ] Bat billing cho GCP project dung.
- [ ] Tao budget alerts o cac moc nho, vi budget khong phai hard spending cap.
- [ ] Chap nhan external IPv4 dang chay co the ton khoang USD 3.60/thang.
- [ ] Theo doi outbound; Compute Engine Free Tier chi co khoang 1 GB/thang.
- [ ] Khong tao snapshot, premium disk, load balancer, Cloud NAT hoac static IP
  thua neu chua hieu chi phi.

## 2. GCP VM

- [ ] Tao non-Spot `e2-micro` trong `us-west1`, `us-central1` hoac `us-east1`.
- [ ] Chon Ubuntu 24.04 LTS amd64.
- [ ] Chon `pd-standard`, toi da 30 GB neu muon nam trong disk allowance.
- [ ] Khong mo 80, 443 hoac 8088.
- [ ] Cho phep SSH 22 de GitHub-hosted Actions deploy.
- [ ] Tao SSH key rieng `shiftz_github_actions`.
- [ ] Upload `deploy/gcp` va public key, sau do chay `bootstrap.sh`.
- [ ] Test login `shiftz-deploy` truoc khi dong phien admin ban dau.

## 3. Runtime Secrets Tren VM

- [ ] Dien `/etc/shiftz/shiftz.env` va giu mode `600`, owner `root:root`.
- [ ] Dung Neon pooled URL cho `DATABASE_URL`.
- [ ] Khong dat Neon direct URL trong runtime env.
- [ ] Tao `SESSION_SECRET` bang `openssl rand -base64 32`.
- [ ] Dien `TELEGRAM_BOT_TOKEN` va `TELEGRAM_WEBHOOK_SECRET`.
- [ ] Dien Google AI Studio key vao `LLM_API_KEY`.
- [ ] Confirm `APP_ADDR=127.0.0.1:8088`.
- [ ] Confirm `COOKIE_SECURE=true`.
- [ ] Confirm `REMINDER_MODE=loop`.
- [ ] Confirm `DEV_API_ENABLED=false`.
- [ ] Chi bat `OWNER_SIGNUP_ENABLED=true` khi muon mo public signup.

## 4. Cloudflare Tunnel

- [ ] Tao remotely managed tunnel cho shiftZ.
- [ ] Tao public hostname route den `http://127.0.0.1:8088`.
- [ ] Cai `cloudflared` truc tiep tren host.
- [ ] Install tunnel thanh systemd service bang token.
- [ ] Confirm `systemctl status cloudflared` dang active.
- [ ] Khong commit hoac dua tunnel token vao GitHub log.

## 5. GitHub Environment

- [ ] Tao Environment ten chinh xac `production`.
- [ ] Them `DEPLOY_HOST`.
- [ ] Them `DEPLOY_USER=shiftz-deploy`.
- [ ] Them private key vao `DEPLOY_SSH_KEY`.
- [ ] Lay SSH host key tu VM console/phien da verify, them vao
  `DEPLOY_KNOWN_HOSTS`.
- [ ] Them Neon direct URL vao `MIGRATION_DATABASE_URL`.
- [ ] Chay workflow `Deploy GCP` lan dau.
- [ ] Confirm Atlas migration xong truoc buoc upload/restart.
- [ ] Confirm artifact `shiftz-linux-amd64-*` xuat hien trong workflow run.

## 6. Telegram Webhook

- [ ] Dang ky `https://<app-domain>/telegram/webhook` bang BotFather token.
- [ ] Gui cung `TELEGRAM_WEBHOOK_SECRET` trong `secret_token`.
- [ ] Goi `getWebhookInfo` va confirm URL dung, khong co recent error.
- [ ] Test `/start <invite-code>` trong private chat.
- [ ] Test parse availability, Confirm va Cancel.
- [ ] Test `/setup <code>` trong Telegram group.

## 7. Smoke Test

- [ ] Local VM `/livez` tra 200.
- [ ] Local VM `/readyz` tra 200.
- [ ] Public `/livez` va `/readyz` tra 200 qua Cloudflare.
- [ ] `/login` load duoc, login sai token bi tu choi.
- [ ] Static CSS va JavaScript load duoc.
- [ ] `/api/...` tra 404 khi `DEV_API_ENABLED=false`.
- [ ] Employee submit availability va dashboard hien submission.
- [ ] Generate va approve schedule thanh cong.
- [ ] Reminder loop khong gui trung khi tick lap lai.

## 8. Van Hanh

- [ ] Xem service: `sudo systemctl status shiftz.service`.
- [ ] Xem log: `journalctl -u shiftz.service -n 100 --no-pager`.
- [ ] Restart: `sudo systemctl restart shiftz.service`.
- [ ] Rollback binary: `sudo /usr/local/sbin/shiftz-rollback`.
- [ ] Nho rang rollback binary khong rollback migration database.
- [ ] Moi migration production phai tuong thich voi binary cu ngay truoc no.
- [ ] Theo doi GCP CPU, RAM, swap, restart, disk va network egress.
- [ ] Theo doi Neon usage va Gemini quota.
- [ ] Rotate deploy key, bot token, webhook secret va API key dinh ky.

## 9. Incident

- [ ] `/livez` fail: xem status va journal cua `shiftz.service`.
- [ ] `/readyz` fail: check Neon status, URL, pool va outbound network.
- [ ] Public fail nhung local healthy: check `cloudflared.service` va DNS.
- [ ] Telegram fail: check `getWebhookInfo` va webhook secret.
- [ ] Deploy fail sau migration: giu binary cu, khong rollback DB tuy tien.
- [ ] OOM/swap lien tuc: nang machine type va dieu chinh systemd limits.
