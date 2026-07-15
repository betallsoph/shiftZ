# Manual Tasks Checklist

Nhung viec phai tu lam bang tay khi dua shiftZ tu local/dev len beta production.

File nay khong thay the README. No la checklist van hanh: tao account, lay key, set env, apply migration, deploy, test, va cac viec chua tu dong hoa.

## 1. Tai Khoan Va Dich Vu Can Tao

- [ ] Tao Neon project cho Postgres production/beta.
- [ ] Lay Neon pooled connection string cho app runtime.
- [ ] Lay Neon direct connection string cho Atlas migrations.
- [ ] Tao Telegram bot bang BotFather.
- [ ] Luu `TELEGRAM_BOT_TOKEN`.
- [ ] Tao `TELEGRAM_WEBHOOK_SECRET` ngau nhien.
- [ ] Tao Google AI Studio / Gemini API key.
- [ ] Chon Gemini model ban dau, vi du `gemini-3.5-flash` hoac model Flash re hon neu dang co.
- [ ] Tao `SESSION_SECRET`:

```sh
openssl rand -base64 32
```

- [ ] (Khuyen nghi) Bat admin portal va dat mat khau admin trong `.env` quyen `600`:

```sh
openssl rand -base64 32   # ADMIN_SESSION_SECRET
```

Canh bao: `ADMIN_PASSWORD` duoc luu plaintext trong `.env`; khong commit, khong log va nen dat mat khau manh.

- [ ] VPS co Docker Engine + Compose plugin va user deploy chay duoc Docker.
- [ ] Them hostname shiftZ vao Cloudflare Tunnel dang co.

## 2. Bien Moi Truong Production

Set cho unified runtime (`cmd/app`):

```sh
DATABASE_URL='Neon pooled URL'
SESSION_SECRET='...'
COOKIE_SECURE=true
TELEGRAM_BOT_TOKEN='...'
TELEGRAM_WEBHOOK_SECRET='...'
LLM_PROVIDER=gemini
LLM_API_KEY='...'
LLM_MODEL='gemini-3.5-flash'
REMINDER_MODE=loop
DEV_API_ENABLED=false
ADMIN_PORTAL_ENABLED=true
ADMIN_USERNAME='...'
ADMIN_PASSWORD='...'
ADMIN_SESSION_SECRET='...'
DB_MAX_OPEN_CONNS=5
DB_MAX_IDLE_CONNS=2
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m
```

Listen address tren VPS:

```sh
APP_ADDR=':8088'
```

Dung rieng cho migration:

```sh
MIGRATION_DATABASE_URL='Neon direct URL'
```

Khong bat `DEV_API_ENABLED` tren production. Chi bat
Tao shop va cap username trong admin portal `/admin`.

VPS always-on chay reminder loop trong cung container. Khong can external
scheduler hay `REMINDER_TRIGGER_SECRET`.

## 3. Database Va Migration

- [ ] Confirm dang dung Neon direct URL cho migration, khong dung pooled URL.
- [ ] Luu Neon direct URL trong file `.env` tren VPS.
- [ ] Workflow deploy se apply migrations **truoc** khi restart app.
- [ ] Neu can fallback bang tay:

```sh
docker compose up -d --build
```

- [ ] Neu co schema change moi:

```sh
go generate ./internal/ent
DEV_DATABASE_URL='postgres://shiftbot:shiftbot@localhost:5432/dev?sslmode=disable' \
  go run ./cmd/migratediff <migration_name>
go test ./...
go vet ./...
docker compose up -d --build
```

- [ ] Check Neon dashboard xem database co tables moi sau migration.
- [ ] Ghi lai Neon project id / branch / region vao noi quan ly rieng.

## 4. Deploy Service Tren VPS

- [ ] Clone repo vao `/home/ubuntu/shiftZ`.
- [ ] Tao `/home/ubuntu/shiftZ/.env` tu `.env.example`, `chmod 600`.
- [ ] Tao GitHub Environment `production` va cac secrets trong
  `deploy/vps/README.md`.
- [ ] Cloudflare Tunnel host route vao `http://127.0.0.1:8088`; khong can Nginx.
- [ ] Push `main` hoac chay workflow `Deploy VPS` bang tay.
- [ ] Confirm Compose build thanh cong va container healthy.
- [ ] Liveness probe dung `/livez`.
- [ ] Readiness probe dung `/readyz`.
- [ ] Khong cau hinh liveness probe vao `/readyz`, vi no ping DB va co the danh thuc Neon.
- [ ] Confirm `/api/...` tra 404 khi `DEV_API_ENABLED=false`.
- [ ] Confirm `/login` mo duoc.
- [ ] Confirm static assets `/static/dashboard.css` va `/static/dashboard.js` load duoc.

## 5. Telegram Webhook

Sau khi app co public HTTPS URL:

```sh
curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://<app-domain>/telegram/webhook" \
  -d "secret_token=$TELEGRAM_WEBHOOK_SECRET"
```

- [ ] Goi `getWebhookInfo` de confirm webhook dung URL:

```sh
curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/getWebhookInfo"
```

- [ ] Gui `/start <invite-code>` tu Telegram user test.
- [ ] Gui availability mau.
- [ ] Confirm bot hoi Confirm/Cancel.
- [ ] Bam Confirm va confirm DB co availability.

## 6. Tao Shop Dau Tien

- [ ] Dang nhap admin portal va tao shop/cap username.
- [ ] Luu lai:
  - Shop ID
  - Dashboard username
  - Invite code
- [ ] Dang nhap `/login` bang username da cap.

- [ ] Hoac tao shop/token bang tool/admin SQL rieng.
- [ ] Dam bao shop co `dashboard_token_hash`.
- [ ] Dam bao shop co shift templates.
- [ ] Luu username cua tung quan trong danh sach van hanh.

## 7. Smoke Test Beta

- [ ] `GET /livez` tra 200.
- [ ] `GET /readyz` tra 200.
- [ ] `/login` sai token bi tu choi.
- [ ] `/login` dung token vao dashboard duoc.
- [ ] Dashboard khong co o nhap `shop_id`.
- [ ] Employee join Telegram bang invite code.
- [ ] Employee gui availability va Confirm.
- [ ] Dashboard hien availability submission status.
- [ ] Generate schedule thanh cong.
- [ ] Approve schedule thanh cong.
- [ ] Generate lai cung shop/week tra duplicate/existing behavior dung.
- [ ] Reminder loop khong spam khi tick lap lai.

## 8. Viec Van Hanh Hang Tuan

- [ ] Check log app sau moc Thursday 10:00 local: reminder da gui.
- [ ] Check log app sau moc Saturday 10:00 local: nag chi gui nguoi chua submit.
- [ ] Check error tu Gemini parsing.
- [ ] Check Telegram webhook errors.
- [ ] Check Neon usage: CU-hours, storage, active connections.
- [ ] Check hosting memory/CPU/restart.
- [ ] Check CI tren GitHub Actions van pass.
- [ ] Check workflow `Deploy VPS` build ARM64, migrate va health-check thanh cong.

## 9. Viec Chua Tu Dong Hoa

- [ ] Doi dashboard username khi chu quan can cap lai quyen truy cap.
- [ ] Doi timezone/shop info tu dashboard.
- [ ] Billing/free-tier enforcement.
- [ ] Backup/restore drill.
- [ ] Alert khi bot webhook fail.
- [ ] Alert khi reminders failed delivery tang cao.
- [ ] Retry policy cho failed reminder delivery.
- [ ] Manual schedule editing.
- [ ] Full auth/account/email flow.

## 10. Khi Co Incident

- [ ] Kiem tra `/livez` de biet process con song khong.
- [ ] Kiem tra `/readyz` de biet DB co ket noi duoc khong.
- [ ] Kiem tra Neon status/dashboard.
- [ ] Kiem tra hosting logs cua unified app.
- [ ] Kiem tra Telegram `getWebhookInfo`.
- [ ] Neu Gemini loi, bot co the khong parse availability moi; thong bao user gui lai sau.
- [ ] Neu DB loi, tam dung deploy/migration va khong generate schedule moi.

## 11. Nguyen Tac An Toan

- [ ] Khong commit `.env` hoac plaintext token.
- [ ] Khong dung `DEV_API_ENABLED=true` o production mac dinh.
- [ ] Khong dung Neon pooled URL cho Atlas migration.
- [ ] Khong point liveness probe vao `/readyz`.
- [ ] Khong log session cookie hoac session secret.
- [ ] Rotating `SESSION_SECRET` se logout tat ca dashboard sessions.
- [ ] Khi test production, dung shop test rieng, khong dung shop khach that.
