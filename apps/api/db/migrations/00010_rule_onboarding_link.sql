-- +goose Up

-- A rule can optionally start an onboarding checklist in addition to enrolling
-- the user in the path's courses.
ALTER TABLE assignment_rules
  ADD COLUMN onboarding_template_id uuid REFERENCES onboarding_templates(id);

-- Backfill the seed rule to point at the Retail first-90 template.
UPDATE assignment_rules
SET onboarding_template_id = 'dddddddd-0000-7000-8000-000000000031'
WHERE id = 'dddddddd-0000-7000-8000-000000000021'
  AND onboarding_template_id IS NULL;

-- +goose Down
ALTER TABLE assignment_rules DROP COLUMN IF EXISTS onboarding_template_id;