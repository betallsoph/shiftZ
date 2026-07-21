-- Modify "shops" table
ALTER TABLE "shops" ADD COLUMN "telegram_team_chat_id" bigint NULL;
ALTER TABLE "shops" ADD COLUMN "owner_telegram_id" bigint NULL;
ALTER TABLE "shops" ADD COLUMN "owner_link_token_hash" character varying NULL;
ALTER TABLE "shops" ADD COLUMN "owner_link_token_expires_at" timestamptz NULL;
-- Create index "shop_owner_telegram_id" to table: "shops"
CREATE UNIQUE INDEX "shop_owner_telegram_id" ON "shops" ("owner_telegram_id");
