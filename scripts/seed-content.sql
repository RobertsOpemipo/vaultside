BEGIN;

INSERT INTO courses (id, tenant_id, title, summary, kind, created_by) VALUES
  ('66666666-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   'First 90 Days at Harbourline', 'Your onboarding path from welcome through to certification.', 'onboarding',
   '44444444-0000-7000-8000-000000000003'),
  ('66666666-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   'AML Essentials', 'Anti-money-laundering fundamentals with a scenario checkpoint.', 'compliance',
   '44444444-0000-7000-8000-000000000003'),
  ('66666666-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   'Data Protection in Practice', 'Handling customer data under Nigeria''s Data Protection Act.', 'compliance',
   '44444444-0000-7000-8000-000000000003'),
  ('66666666-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   'Phishing and Cyber Hygiene', 'Six-monthly cyber awareness refresh.', 'compliance',
   '44444444-0000-7000-8000-000000000003'),
  ('66666666-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   'Customer Conversations', 'Skills for relationship officers.', 'skills',
   '44444444-0000-7000-8000-000000000003')
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_versions (id, tenant_id, course_id, version, status, authored_by, reviewed_by, published_at) VALUES
  ('77777777-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   '66666666-0000-7000-8000-000000000001', 1, 'published',
   '44444444-0000-7000-8000-000000000003', '44444444-0000-7000-8000-000000000005', now()),
  ('77777777-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   '66666666-0000-7000-8000-000000000002', 1, 'published',
   '44444444-0000-7000-8000-000000000003', '44444444-0000-7000-8000-000000000005', now()),
  ('77777777-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   '66666666-0000-7000-8000-000000000003', 1, 'published',
   '44444444-0000-7000-8000-000000000003', '44444444-0000-7000-8000-000000000005', now()),
  ('77777777-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   '66666666-0000-7000-8000-000000000004', 1, 'published',
   '44444444-0000-7000-8000-000000000003', '44444444-0000-7000-8000-000000000005', now()),
  ('77777777-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   '66666666-0000-7000-8000-000000000005', 1, 'published',
   '44444444-0000-7000-8000-000000000003', '44444444-0000-7000-8000-000000000005', now())
ON CONFLICT (id) DO NOTHING;

UPDATE courses SET current_version_id = cv.id
FROM course_versions cv
WHERE cv.course_id = courses.id
  AND cv.status = 'published'
  AND courses.current_version_id IS NULL;

INSERT INTO lessons (id, tenant_id, version_id, position, title, kind, content_md, duration_s) VALUES
  ('88888888-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 1,
   'Welcome to Harbourline', 'rich_text',
   E'# Welcome to Harbourline\n\nYou are joining a bank that has served Nigerian families and businesses for 40 years. Over your first 90 days you will learn how we work, what we stand for, and how to keep our customers - and yourself - safe.\n\nThis path is a map, not a race. Complete one lesson at a time.',
   240),
  ('88888888-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 2,
   'How to find help', 'rich_text',
   E'# How to find help\n\nNobody expects you to know everything in week one. Here is who to ask.\n\n- **Your manager** for anything about your role or workload.\n- **Your buddy** - a peer assigned on day one.\n- **The service desk** for systems, login, and access.\n- **Compliance and Risk** for anything that feels off.\n\nIf you are unsure whether something is a compliance issue, report it anyway.',
   180),
  ('88888888-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 3,
   'Code of conduct', 'rich_text',
   E'# Code of conduct\n\nHarbourline expects every member of staff to act with integrity, in the interest of customers, and within the law.\n\n**Conflicts of interest** must be declared and managed.\n\n**Gifts and hospitality** over 25,000 naira must be logged. Anything that could look like an inducement must be refused and reported.\n\n**Confidentiality** applies to customer data and internal information.',
   300),
  ('88888888-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 4,
   'Know your customer', 'rich_text',
   E'# Know your customer\n\nBefore we serve a customer, we must answer three questions.\n\n1. **Who is this person?** Verified identity, not just a name.\n2. **What will they use this account for?** Expected activity, income, source of funds.\n3. **Does that make sense?** A student with a 500,000 naira monthly turnover needs a conversation.\n\nIf the answers change later, so does our risk.',
   420),
  ('88888888-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 5,
   'Money laundering basics', 'rich_text',
   E'# Money laundering basics\n\nMoney laundering is the process of making criminal money look legitimate. It happens in three stages.\n\n**Placement** - getting cash into the financial system.\n\n**Layering** - moving money to obscure its origin.\n\n**Integration** - the money re-enters the legitimate economy.\n\nYour job is not to catch the launderer. Your job is to notice, escalate, and never tip off the customer.',
   360),
  ('88888888-0000-7000-8000-000000000006', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 6,
   'Data and cyber hygiene', 'rich_text',
   E'# Data and cyber hygiene\n\nCustomer data is a liability as much as an asset.\n\n**Never** send customer data over personal email or WhatsApp.\n\n**Always** lock your screen when you stand up.\n\n**Report** every suspicious email, even if you did not click.',
   300),
  ('88888888-0000-7000-8000-000000000007', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 7,
   'Scenario checkpoint', 'quiz', NULL, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, tenant_id, version_id, position, title, kind, content_md, duration_s) VALUES
  ('88888888-0000-7000-8000-000000000101', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000002', 1,
   'Why AML matters', 'rich_text',
   E'# Why AML matters\n\nA bank that cannot detect money laundering becomes a tool for it. Regulators, correspondent banks, and the public all watch.\n\nAML is not paperwork. It is the work of keeping the financial system clean.',
   240),
  ('88888888-0000-7000-8000-000000000102', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000002', 2,
   'Red flags', 'rich_text',
   E'# Red flags\n\nRed flags are signals, not proof. Two or three together warrant escalation.\n\n- Large cash deposits inconsistent with profile\n- Structuring - deposits just under reporting thresholds\n- A customer who cannot explain the source of funds\n- Third parties depositing on behalf of the account holder\n- Unusual urgency, evasiveness, or hostility',
   360),
  ('88888888-0000-7000-8000-000000000103', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000002', 3,
   'AML checkpoint', 'quiz', NULL, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, tenant_id, version_id, position, title, kind, content_md, duration_s) VALUES
  ('88888888-0000-7000-8000-000000000201', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000003', 1,
   'Customer data principles', 'rich_text',
   E'# Customer data principles\n\nCustomers have rights over their data. We have obligations.\n\n**Lawful basis** - we collect data for a specific, legitimate purpose.\n\n**Minimisation** - we collect what we need, not what we might want.\n\n**Retention** - we keep it only as long as necessary.\n\n**Security** - we protect it from unauthorised access.',
   300),
  ('88888888-0000-7000-8000-000000000202', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000003', 2,
   'Data protection checkpoint', 'quiz', NULL, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, tenant_id, version_id, position, title, kind, content_md, duration_s) VALUES
  ('88888888-0000-7000-8000-000000000301', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000004', 1,
   'Spotting a phishing email', 'rich_text',
   E'# Spotting a phishing email\n\nPhishing has become sophisticated. Look for:\n\n- Sender domain close but not quite right\n- Urgency that makes you want to act before thinking\n- A link whose visible text and destination disagree\n- An attachment you were not expecting\n\nWhen in doubt, verify out-of-band. Call the person on a number you already know.',
   240),
  ('88888888-0000-7000-8000-000000000302', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000004', 2,
   'Phishing checkpoint', 'quiz', NULL, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, tenant_id, version_id, position, title, kind, content_md, duration_s) VALUES
  ('88888888-0000-7000-8000-000000000401', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000005', 1,
   'Listening before selling', 'rich_text',
   E'# Listening before selling\n\nA relationship officer who talks first learns nothing. Ask open questions, reflect back what you heard, and only then introduce a product.\n\n**Open questions** begin with what, how, when. Not "do you want...".\n\n**Silence** is a tool. Give the customer space to think.',
   300),
  ('88888888-0000-7000-8000-000000000402', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000005', 2,
   'Customer Conversations checkpoint', 'quiz', NULL, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO assessments (id, tenant_id, version_id, title, pass_mark, max_attempts, shuffle, pool_size) VALUES
  ('99999999-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   '77777777-0000-7000-8000-000000000001', 'First 90 Days: scenario checkpoint',
   80, 3, true, NULL)
ON CONFLICT (id) DO NOTHING;

INSERT INTO questions (id, tenant_id, assessment_id, position, kind, prompt_md, explanation_md, points) VALUES
  ('aaaaaaaa-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   '99999999-0000-7000-8000-000000000001', 1, 'scenario',
   E'A new customer wants to deposit 2,400,000 naira in cash. When you ask where the money came from, they become evasive and say "it is just savings". What is your first step?',
   E'Tipping off the customer is itself a breach. Follow the escalation procedure and file an internal report without telling them why.',
   1),
  ('aaaaaaaa-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   '99999999-0000-7000-8000-000000000001', 2, 'multiple_choice',
   E'Which of these is a red flag for money laundering?',
   E'All are red flags, but a customer who cannot explain the source of funds is the strongest single signal.',
   1),
  ('aaaaaaaa-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   '99999999-0000-7000-8000-000000000001', 3, 'multiple_choice',
   E'A colleague asks you to share a customer statement "just to check something". What do you do?',
   E'Confidentiality is not optional. If they have a business need, they can request access through the proper channel.',
   1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO question_options (id, tenant_id, question_id, position, text_md, is_correct, feedback_md) VALUES
  ('bbbbbbbb-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000001', 1,
   'Process it quietly to keep the customer happy', false,
   'Never. This would make you part of the laundering chain.'),
  ('bbbbbbbb-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000001', 2,
   'Refuse and tell the customer you suspect them', false,
   'Tipping off is itself an offence under AML rules.'),
  ('bbbbbbbb-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000001', 3,
   'Follow the escalation procedure and file an internal report without tipping off the customer', true,
   'Correct. Escalate, document, and stay silent with the customer.'),
  ('bbbbbbbb-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000001', 4,
   'Ask a colleague to take over', false,
   'Handing it off does not remove the obligation to report.'),
  ('bbbbbbbb-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000002', 1,
   'A customer who cannot explain the source of funds', true,
   'The strongest single signal.'),
  ('bbbbbbbb-0000-7000-8000-000000000006', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000002', 2,
   'Repeated cash deposits just under the reporting threshold', true,
   'This is structuring, a classic red flag.'),
  ('bbbbbbbb-0000-7000-8000-000000000007', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000002', 3,
   'A long-quiet account that suddenly moves large sums', true,
   'Sudden changes in pattern always warrant review.'),
  ('bbbbbbbb-0000-7000-8000-000000000008', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000002', 4,
   'A regular salary deposit from a known employer', false,
   'That is expected activity, not a red flag.'),
  ('bbbbbbbb-0000-7000-8000-000000000009', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000003', 1,
   'Send it. They are a colleague.', false,
   'Colleagues are not a confidentiality exception.'),
  ('bbbbbbbb-0000-7000-8000-00000000000a', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000003', 2,
   'Ask them to request access through the proper channel', true,
   'Correct. Business need must be documented and approved.'),
  ('bbbbbbbb-0000-7000-8000-00000000000b', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000003', 3,
   'Tell your manager and let them decide', false,
   'Escalation is good, but the correct path is the access request process.'),
  ('bbbbbbbb-0000-7000-8000-00000000000c', '11111111-1111-7111-8111-111111111111',
   'aaaaaaaa-0000-7000-8000-000000000003', 4,
   'Share a redacted copy', false,
   'Redaction does not remove the confidentiality obligation.')
ON CONFLICT (id) DO NOTHING;

COMMIT;

SELECT
  (SELECT count(*) FROM courses          WHERE tenant_id = '11111111-1111-7111-8111-111111111111') AS courses,
  (SELECT count(*) FROM course_versions  WHERE tenant_id = '11111111-1111-7111-8111-111111111111') AS versions,
  (SELECT count(*) FROM lessons          WHERE tenant_id = '11111111-1111-7111-8111-111111111111') AS lessons,
  (SELECT count(*) FROM assessments      WHERE tenant_id = '11111111-1111-7111-8111-111111111111') AS assessments,
  (SELECT count(*) FROM questions        WHERE tenant_id = '11111111-1111-7111-8111-111111111111') AS questions,
  (SELECT count(*) FROM question_options WHERE tenant_id = '11111111-1111-7111-8111-111111111111') AS options;