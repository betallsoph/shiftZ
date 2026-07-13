-- Modify "shops" table
ALTER TABLE "shops" ADD COLUMN "telegram_setup_code_hash" character varying NULL, ADD COLUMN "telegram_setup_code_expires_at" timestamptz NULL;
-- Create index "shop_telegram_setup_code_hash" to table: "shops"
CREATE UNIQUE INDEX "shop_telegram_setup_code_hash" ON "shops" ("telegram_setup_code_hash");
