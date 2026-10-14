BEGIN;

-- ---------- Learning path: New Hire Retail ----------
INSERT INTO learning_paths (id, tenant_id, key, title, description, kind) VALUES
  ('dddddddd-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   'new-hire-retail', 'New Hire Retail',
   'First 90 days for new Retail Banking staff.', 'onboarding')
ON CONFLICT (id) DO NOTHING;

-- Path items: First 90 Days, then AML Essentials
INSERT INTO path_items (id, tenant_id, path_id, position, course_id, is_required) VALUES
  ('dddddddd-0000-7000-8000-000000000011', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000001', 1,
   '66666666-0000-7000-8000-000000000001', true),
  ('dddddddd-0000-7000-8000-000000000012', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000001', 2,
   '66666666-0000-7000-8000-000000000002', true)
ON CONFLICT (id) DO NOTHING;

-- ---------- Assignment rule: Retail hires in their first 7 days ----------
-- department_id is the Retail Banking department (2222...0001)
-- hired_within_days matches users whose hired_on is within the last 7 days
INSERT INTO assignment_rules (id, tenant_id, name, path_id, priority, is_active, conditions) VALUES
  ('dddddddd-0000-7000-8000-000000000021', '11111111-1111-7111-8111-111111111111',
   'Retail hires — first 7 days',
   'dddddddd-0000-7000-8000-000000000001',
   10, true,
   '{"and":[{"field":"department_id","op":"eq","value":"22222222-0000-7000-8000-000000000001"},{"field":"hired_within_days","op":"lte","value":7}]}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- ---------- Onboarding template: standard Retail first-90-days checklist ----------
INSERT INTO onboarding_templates (id, tenant_id, key, title, description) VALUES
  ('dddddddd-0000-7000-8000-000000000031', '11111111-1111-7111-8111-111111111111',
   'retail-first-90', 'Retail first 90 days',
   'Manager checklist for new Retail Banking hires.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO onboarding_items (id, tenant_id, template_id, position, title, description, requires_manager_approval) VALUES
  ('dddddddd-0000-7000-8000-000000000041', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000031', 1,
   'Desk and system access', 'Badge issued, network login confirmed, core banking access provisioned.', true),
  ('dddddddd-0000-7000-8000-000000000042', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000031', 2,
   'Meet your buddy', 'Introduce the new hire to their assigned buddy.', false),
  ('dddddddd-0000-7000-8000-000000000043', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000031', 3,
   'Complete Week 1 training', 'First 90 Days course, Week 1 lessons and Code of Conduct attestation.', true),
  ('dddddddd-0000-7000-8000-000000000044', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000031', 4,
   '30-day check-in', 'Sit with the new hire and confirm they are comfortable in the role.', true),
  ('dddddddd-0000-7000-8000-000000000045', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000031', 5,
   '60-day check-in', 'Review progress against the 60-day milestones.', true),
  ('dddddddd-0000-7000-8000-000000000046', '11111111-1111-7111-8111-111111111111',
   'dddddddd-0000-7000-8000-000000000031', 6,
   '90-day review', 'Formal 90-day review. Sign off the onboarding checklist.', true)
ON CONFLICT (id) DO NOTHING;

COMMIT;

SELECT
  (SELECT count(*) FROM learning_paths)       AS paths,
  (SELECT count(*) FROM path_items)           AS items,
  (SELECT count(*) FROM assignment_rules)     AS rules,
  (SELECT count(*) FROM onboarding_templates) AS templates,
  (SELECT count(*) FROM onboarding_items)     AS checklist_items;