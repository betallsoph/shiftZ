-- Drop index "shop_telegram_setup_code_hash" from table: "shops"
DROP INDEX "shop_telegram_setup_code_hash";
-- Modify "shops" table
ALTER TABLE "shops" DROP COLUMN "telegram_setup_code_hash", DROP COLUMN "telegram_setup_code_expires_at";
