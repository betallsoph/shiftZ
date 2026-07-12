-- Create index "schedule_shop_id_week_start_variant_label" to table: "schedules"
CREATE UNIQUE INDEX "schedule_shop_id_week_start_variant_label" ON "schedules" ("shop_id", "week_start", "variant_label");
