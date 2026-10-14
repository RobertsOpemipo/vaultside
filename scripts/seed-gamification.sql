BEGIN;

-- ---------- Badges ----------
INSERT INTO badges (id, tenant_id, key, name, description, category, points_value, theme_meta) VALUES
  ('bbbbbbbb-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   'first_lesson', 'First Steps',
   'Completed your first lesson.', 'milestone', 0,
   '{"icon": "footprint", "color": "harbour"}'),
  ('bbbbbbbb-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   'first_course', 'Trailblazer',
   'Completed your first course.', 'milestone', 0,
   '{"icon": "flag", "color": "marigold"}'),
  ('bbbbbbbb-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   'quiz_ace', 'Perfect Score',
   'Passed a checkpoint with 100 percent.', 'behavior', 0,
   '{"icon": "star", "color": "marigold"}'),
  ('bbbbbbbb-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   'streak_7', 'Steady Hand',
   'Kept a 7-day learning streak.', 'behavior', 0,
   '{"icon": "flame", "color": "vermilion"}'),
  ('bbbbbbbb-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   'xp_500', 'Apprentice',
   'Earned 500 XP.', 'milestone', 0,
   '{"icon": "compass", "color": "cobalt"}'),
  ('bbbbbbbb-0000-7000-8000-000000000006', '11111111-1111-7111-8111-111111111111',
   'xp_1000', 'Navigator',
   'Earned 1,000 XP.', 'milestone', 0,
   '{"icon": "map", "color": "cobalt"}'),
  ('bbbbbbbb-0000-7000-8000-000000000007', '11111111-1111-7111-8111-111111111111',
   'three_lessons', 'Finding Your Pace',
   'Completed three lessons.', 'milestone', 0,
   '{"icon": "route", "color": "moss"}')
ON CONFLICT (id) DO NOTHING;

COMMIT;

SELECT count(*) AS badges FROM badges WHERE tenant_id = '11111111-1111-7111-8111-111111111111';