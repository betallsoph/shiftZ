-- Create "shops" table
CREATE TABLE "shops" ("id" uuid NOT NULL, "name" character varying NOT NULL, "timezone" character varying NOT NULL DEFAULT 'UTC', "invite_code" character varying NOT NULL, "telegram_group_id" bigint NOT NULL, "plan" character varying NOT NULL DEFAULT 'free', "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- Create index "shops_invite_code_key" to table: "shops"
CREATE UNIQUE INDEX "shops_invite_code_key" ON "shops" ("invite_code");
-- Create "employees" table
CREATE TABLE "employees" ("id" uuid NOT NULL, "telegram_user_id" bigint NOT NULL, "display_name" character varying NOT NULL, "role" character varying NOT NULL DEFAULT '', "max_hours_per_week" double precision NOT NULL DEFAULT 40, "is_active" boolean NOT NULL DEFAULT true, "created_at" timestamptz NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "employees_shops_employees" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "employee_shop_id_is_active" to table: "employees"
CREATE INDEX "employee_shop_id_is_active" ON "employees" ("shop_id", "is_active");
-- Create index "employee_shop_id_telegram_user_id" to table: "employees"
CREATE UNIQUE INDEX "employee_shop_id_telegram_user_id" ON "employees" ("shop_id", "telegram_user_id");
-- Create index "employee_telegram_user_id" to table: "employees"
CREATE INDEX "employee_telegram_user_id" ON "employees" ("telegram_user_id");
-- Create "availabilities" table
CREATE TABLE "availabilities" ("id" uuid NOT NULL, "week_start" date NOT NULL, "slots" jsonb NOT NULL, "raw_message" text NOT NULL DEFAULT '', "created_at" timestamptz NOT NULL, "employee_id" uuid NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "availabilities_employees_availabilities" FOREIGN KEY ("employee_id") REFERENCES "employees" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "availabilities_shops_availabilities" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "availability_shop_id_employee_id_week_start" to table: "availabilities"
CREATE UNIQUE INDEX "availability_shop_id_employee_id_week_start" ON "availabilities" ("shop_id", "employee_id", "week_start");
-- Create index "availability_shop_id_week_start" to table: "availabilities"
CREATE INDEX "availability_shop_id_week_start" ON "availabilities" ("shop_id", "week_start");
-- Create "rules" table
CREATE TABLE "rules" ("id" uuid NOT NULL, "description" text NOT NULL DEFAULT '', "rule_json" jsonb NULL, "weight" double precision NOT NULL DEFAULT 1, "is_active" boolean NOT NULL DEFAULT true, "created_at" timestamptz NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "rules_shops_rules" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "rule_shop_id_is_active" to table: "rules"
CREATE INDEX "rule_shop_id_is_active" ON "rules" ("shop_id", "is_active");
-- Create "schedules" table
CREATE TABLE "schedules" ("id" uuid NOT NULL, "week_start" date NOT NULL, "status" character varying NOT NULL DEFAULT 'draft', "variant_label" character varying NOT NULL DEFAULT '', "score" double precision NOT NULL DEFAULT 0, "created_at" timestamptz NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "schedules_shops_schedules" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "schedule_shop_id_week_start" to table: "schedules"
CREATE INDEX "schedule_shop_id_week_start" ON "schedules" ("shop_id", "week_start");
-- Create "shifts" table
CREATE TABLE "shifts" ("id" uuid NOT NULL, "name" character varying NOT NULL, "weekday" bigint NOT NULL, "start_time" character varying NOT NULL, "end_time" character varying NOT NULL, "min_staff" bigint NOT NULL DEFAULT 1, "max_staff" bigint NOT NULL DEFAULT 1, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "shifts_shops_shifts" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create "schedule_assignments" table
CREATE TABLE "schedule_assignments" ("id" uuid NOT NULL, "date" date NOT NULL, "employee_id" uuid NOT NULL, "schedule_id" uuid NOT NULL, "shop_id" uuid NOT NULL, "shift_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "schedule_assignments_employees_assignments" FOREIGN KEY ("employee_id") REFERENCES "employees" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "schedule_assignments_schedules_assignments" FOREIGN KEY ("schedule_id") REFERENCES "schedules" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "schedule_assignments_shifts_assignments" FOREIGN KEY ("shift_id") REFERENCES "shifts" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "schedule_assignments_shops_shop" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "scheduleassignment_shop_id_employee_id_date" to table: "schedule_assignments"
CREATE INDEX "scheduleassignment_shop_id_employee_id_date" ON "schedule_assignments" ("shop_id", "employee_id", "date");
-- Create index "scheduleassignment_shop_id_schedule_id" to table: "schedule_assignments"
CREATE INDEX "scheduleassignment_shop_id_schedule_id" ON "schedule_assignments" ("shop_id", "schedule_id");
-- Create "schedule_votes" table
CREATE TABLE "schedule_votes" ("id" uuid NOT NULL, "week_start" date NOT NULL, "created_at" timestamptz NOT NULL, "employee_id" uuid NOT NULL, "schedule_id" uuid NOT NULL, "shop_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "schedule_votes_employees_votes" FOREIGN KEY ("employee_id") REFERENCES "employees" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "schedule_votes_schedules_votes" FOREIGN KEY ("schedule_id") REFERENCES "schedules" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "schedule_votes_shops_shop" FOREIGN KEY ("shop_id") REFERENCES "shops" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- Create index "schedulevote_shop_id_employee_id_week_start" to table: "schedule_votes"
CREATE UNIQUE INDEX "schedulevote_shop_id_employee_id_week_start" ON "schedule_votes" ("shop_id", "employee_id", "week_start");
-- Create index "schedulevote_shop_id_schedule_id" to table: "schedule_votes"
CREATE INDEX "schedulevote_shop_id_schedule_id" ON "schedule_votes" ("shop_id", "schedule_id");
