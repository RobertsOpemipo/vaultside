BEGIN;

INSERT INTO quests (id, tenant_id, key, title, description, cadence, kind, target_value, xp_reward, theme_meta) VALUES
  ('cccccccc-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   'daily_one_lesson', 'Today''s Step',
   'Complete one lesson today.', 'daily', 'lesson', 1, 5,
   '{"icon": "footprint", "color": "harbour"}'),
  ('cccccccc-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   'weekly_three_lessons', 'This Week''s Pace',
   'Complete three lessons this week.', 'weekly', 'lesson', 3, 20,
   '{"icon": "route", "color": "moss"}'),
  ('cccccccc-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   'weekly_one_quiz', 'Weekly Checkpoint',
   'Pass one quiz this week.', 'weekly', 'quiz_pass', 1, 30,
   '{"icon": "star", "color": "marigold"}'),
  ('cccccccc-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   'onboarding_first_lesson', 'Departing the Dock',
   'Complete your very first lesson.', 'onboarding', 'lesson', 1, 15,
   '{"icon": "anchor", "color": "harbour"}'),
  ('cccccccc-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   'onboarding_first_quiz', 'First Checkpoint',
   'Pass your first quiz.', 'onboarding', 'quiz_pass', 1, 25,
   '{"icon": "checkpoint", "color": "marigold"}')
ON CONFLICT (id) DO NOTHING;

COMMIT;

SELECT count(*) AS quests FROM quests WHERE tenant_id = '11111111-1111-7111-8111-111111111111';