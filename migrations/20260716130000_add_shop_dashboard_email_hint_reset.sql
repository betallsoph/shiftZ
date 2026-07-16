-- Modify "shops" table
ALTER TABLE "shops" ADD COLUMN "dashboard_email" character varying NULL;
ALTER TABLE "shops" ADD COLUMN "dashboard_password_hint" character varying NULL;
ALTER TABLE "shops" ADD COLUMN "dashboard_password_reset_hash" character varying NULL;
ALTER TABLE "shops" ADD COLUMN "dashboard_password_reset_expires_at" timestamptz NULL;
