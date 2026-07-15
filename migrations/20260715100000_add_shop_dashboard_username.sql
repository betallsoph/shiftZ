-- Modify "shops" table
ALTER TABLE "shops" ADD COLUMN "dashboard_username" character varying NULL;
-- Create index "shop_dashboard_username" to table: "shops"
CREATE UNIQUE INDEX "shop_dashboard_username" ON "shops" ("dashboard_username");
