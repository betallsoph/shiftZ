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

- [ ] Chon hosting cho `cmd/server`.
- [ ] Chon hosting cho `cmd/bot`.
- [ ] Tro domain/HTTPS neu deploy public beta.

## 2. Bien Moi Truong Production

Set cho `cmd/server`:

```sh
DATABASE_URL='Neon pooled URL'
SERVER_ADDR=':8080'
SESSION_SECRET='...'
COOKIE_SECURE=true
DEV_API_ENABLED=false
DB_MAX_OPEN_CONNS=5
DB_MAX_IDLE_CONNS=2
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m
```

Set cho `cmd/bot`:

```sh
DATABASE_URL='Neon pooled URL'
BOT_ADDR=':8081'
TELEGRAM_BOT_TOKEN='...'
TELEGRAM_WEBHOOK_SECRET='...'
LLM_PROVIDER=gemini
LLM_API_KEY='...'
LLM_MODEL='gemini-3.5-flash'
REMINDERS_ENABLED=true
REMINDER_TICK_INTERVAL=1m
DB_MAX_OPEN_CONNS=5
DB_MAX_IDLE_CONNS=2
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m
```

Dung rieng cho migration:

```sh
MIGRATION_DATABASE_URL='Neon direct URL'
```

Khong bat `DEV_API_ENABLED` tren production tru khi dang debug co chu dich.

## 3. Database Va Migration

- [ ] Confirm dang dung Neon direct URL cho migration, khong dung pooled URL.
- [ ] Apply migrations:

```sh
atlas migrate apply --dir file://migrations --url "$MIGRATION_DATABASE_URL"
```

- [ ] Neu co schema change moi:

```sh
go generate ./internal/ent
DEV_DATABASE_URL='postgres://shiftbot:shiftbot@localhost:5432/dev?sslmode=disable' \
  go run ./cmd/migratediff <migration_name>
go test ./...
go vet ./...
atlas migrate apply --dir file://migrations --url "$MIGRATION_DATABASE_URL"
```

- [ ] Check Neon dashboard xem database co tables moi sau migration.
- [ ] Ghi lai Neon project id / branch / region vao noi quan ly rieng.

## 4. Deploy Services

- [ ] Deploy `cmd/server`.
- [ ] Deploy `cmd/bot`.
- [ ] Liveness probe dung `/livez`.
- [ ] Readiness probe dung `/readyz`.
- [ ] Khong cau hinh liveness probe vao `/readyz`, vi no ping DB va co the danh thuc Neon.
- [ ] Confirm `/api/...` tra 404 khi `DEV_API_ENABLED=false`.
- [ ] Confirm `/login` mo duoc tren server.
- [ ] Confirm static assets `/static/dashboard.css` va `/static/dashboard.js` load duoc.

## 5. Telegram Webhook

Sau khi `cmd/bot` co public HTTPS URL:

```sh
curl "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d "url=https://<bot-domain>/telegram/webhook" \
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

Neu owner onboarding da duoc deploy:

- [ ] Bat `OWNER_SIGNUP_ENABLED=true` neu signup duoc gate bang env.
- [ ] Vao `/signup`.
- [ ] Tao shop.
- [ ] Luu lai:
  - Shop ID
  - Owner dashboard token
  - Invite code
- [ ] Dang nhap `/login` bang Shop ID + owner token.

Neu owner onboarding chua duoc deploy:

- [ ] Chay seed tren moi truong local/staging de demo:

```sh
go run ./cmd/seed
```

- [ ] Hoac tao shop/token bang tool/admin SQL rieng.
- [ ] Dam bao shop co `dashboard_token_hash`.
- [ ] Dam bao shop co shift templates.
- [ ] Luu owner token o password manager. Plaintext token chi nen xuat hien mot lan.

## 7. Smoke Test Beta

- [ ] `GET /livez` cua `cmd/server` tra 200.
- [ ] `GET /readyz` cua `cmd/server` tra 200.
- [ ] `GET /livez` cua `cmd/bot` tra 200.
- [ ] `GET /readyz` cua `cmd/bot` tra 200.
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

- [ ] Check log `cmd/bot` sau moc Thursday 10:00 local: reminder da gui.
- [ ] Check log `cmd/bot` sau moc Saturday 10:00 local: nag chi gui nguoi chua submit.
- [ ] Check error tu Gemini parsing.
- [ ] Check Telegram webhook errors.
- [ ] Check Neon usage: CU-hours, storage, active connections.
- [ ] Check hosting memory/CPU/restart.
- [ ] Check CI tren GitHub Actions van pass.

## 9. Viec Chua Tu Dong Hoa

- [ ] Reset owner dashboard token khi chu quan lam mat token.
- [ ] Doi timezone/shop info tu dashboard.
- [ ] Tao/sua/xoa employee tu dashboard.
- [ ] Tao/sua/xoa shift templates tu dashboard.
- [ ] Gan Telegram group cho shop that.
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
- [ ] Kiem tra hosting logs cua `cmd/server`.
- [ ] Kiem tra hosting logs cua `cmd/bot`.
- [ ] Kiem tra Telegram `getWebhookInfo`.
- [ ] Neu Gemini loi, bot co the khong parse availability moi; thong bao user gui lai sau.
- [ ] Neu DB loi, tam dung deploy/migration va khong generate schedule moi.

## 11. Nguyen Tac An Toan

- [ ] Khong commit `.env` hoac plaintext token.
- [ ] Khong dung `DEV_API_ENABLED=true` o production mac dinh.
- [ ] Khong dung Neon pooled URL cho Atlas migration.
- [ ] Khong point liveness probe vao `/readyz`.
- [ ] Khong log owner dashboard token sau khi tao xong.
- [ ] Rotating `SESSION_SECRET` se logout tat ca dashboard sessions.
- [ ] Khi test production, dung shop test rieng, khong dung shop khach that.

