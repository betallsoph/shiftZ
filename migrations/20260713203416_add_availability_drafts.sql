-- Create "availability_drafts" table
CREATE TABLE "availability_drafts" ("id" uuid NOT NULL, "telegram_user_id" bigint NOT NULL, "chat_id" bigint NOT NULL, "week_start" date NOT NULL, "timezone" character varying NOT NULL, "slots" jsonb NOT NULL, "raw_message" text NOT NULL DEFAULT '', "created_at" timestamptz NOT NULL, "expires_at" timestamptz NOT NULL, "employee_id" uuid NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "availability_drafts_employees_availability_drafts" FOREIGN KEY ("employee_id") REFERENCES "employees" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "availability_drafts_shops_availability_drafts" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "availabilitydraft_expires_at" to table: "availability_drafts"
CREATE INDEX "availabilitydraft_expires_at" ON "availability_drafts" ("expires_at");
-- Create index "availabilitydraft_telegram_user_id_expires_at" to table: "availability_drafts"
CREATE INDEX "availabilitydraft_telegram_user_id_expires_at" ON "availability_drafts" ("telegram_user_id", "expires_at");
