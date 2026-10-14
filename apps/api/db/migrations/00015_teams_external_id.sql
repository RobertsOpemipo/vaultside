-- +goose Up

ALTER TABLE teams ADD COLUMN external_id text;
CREATE UNIQUE INDEX teams_tenant_external_idx
  ON teams (tenant_id, external_id)
  WHERE external_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS teams_tenant_external_idx;
ALTER TABLE teams DROP COLUMN IF EXISTS external_id;