-- Create "reminder_deliveries" table
CREATE TABLE "reminder_deliveries" ("id" uuid NOT NULL, "week_start" date NOT NULL, "kind" character varying NOT NULL, "status" character varying NOT NULL DEFAULT 'pending', "attempts" bigint NOT NULL DEFAULT 0, "last_error" text NULL, "created_at" timestamptz NOT NULL, "sent_at" timestamptz NULL, "employee_id" uuid NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "reminder_deliveries_employees_reminder_deliveries" FOREIGN KEY ("employee_id") REFERENCES "employees" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "reminder_deliveries_shops_reminder_deliveries" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "reminderdelivery_shop_id_employee_id_week_start_kind" to table: "reminder_deliveries"
CREATE UNIQUE INDEX "reminderdelivery_shop_id_employee_id_week_start_kind" ON "reminder_deliveries" ("shop_id", "employee_id", "week_start", "kind");
-- Create index "reminderdelivery_status_created_at" to table: "reminder_deliveries"
CREATE INDEX "reminderdelivery_status_created_at" ON "reminder_deliveries" ("status", "created_at");
