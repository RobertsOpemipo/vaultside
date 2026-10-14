-- +goose Up

-- Per-tenant switch for the AI assist side panel. Off by default. Only a
-- user holding the `tenant.manage` permission (the tenant owner role) can
-- flip it via PATCH /v1/admin/assist.
ALTER TABLE tenants
  ADD COLUMN assist_enabled boolean NOT NULL DEFAULT false;

-- Column-level UPDATE so app_rw can flip the flag even if it only has
-- table-level SELECT on tenants. Redundant if table-level UPDATE already
-- exists; harmless either way.
GRANT SELECT, UPDATE (assist_enabled) ON tenants TO app_rw;

-- +goose Down
ALTER TABLE tenants DROP COLUMN assist_enabled;