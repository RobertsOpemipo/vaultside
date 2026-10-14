BEGIN;

INSERT INTO regulations (id, tenant_id, key, name, description, authority, version) VALUES
  ('ffffffff-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   'aml', 'Anti-Money Laundering',
   'Detection and reporting of money laundering and terrorist financing.',
   'Central bank / financial intelligence unit', 1),
  ('ffffffff-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   'kyc', 'Know Your Customer',
   'Customer identification, verification and ongoing due diligence.',
   'Central bank / financial intelligence unit', 1),
  ('ffffffff-0000-7000-8000-000000000003', '11111111-1111-7111-8111-111111111111',
   'dp', 'Data Protection',
   'Protection of personal data and customer privacy.',
   'Data protection authority', 1),
  ('ffffffff-0000-7000-8000-000000000004', '11111111-1111-7111-8111-111111111111',
   'conduct', 'Conduct and Fair Treatment',
   'Fair treatment of customers and market conduct standards.',
   'Financial services regulator', 1),
  ('ffffffff-0000-7000-8000-000000000005', '11111111-1111-7111-8111-111111111111',
   'infosec', 'Information Security',
   'Cyber hygiene, phishing resistance, and protective controls.',
   'Central bank / information security regulator', 1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO course_regulations (tenant_id, course_id, regulation_id) VALUES
  ('11111111-1111-7111-8111-111111111111', '66666666-0000-7000-8000-000000000001', 'ffffffff-0000-7000-8000-000000000004'),
  ('11111111-1111-7111-8111-111111111111', '66666666-0000-7000-8000-000000000001', 'ffffffff-0000-7000-8000-000000000002'),
  ('11111111-1111-7111-8111-111111111111', '66666666-0000-7000-8000-000000000002', 'ffffffff-0000-7000-8000-000000000001'),
  ('11111111-1111-7111-8111-111111111111', '66666666-0000-7000-8000-000000000003', 'ffffffff-0000-7000-8000-000000000003'),
  ('11111111-1111-7111-8111-111111111111', '66666666-0000-7000-8000-000000000004', 'ffffffff-0000-7000-8000-000000000005'),
  ('11111111-1111-7111-8111-111111111111', '66666666-0000-7000-8000-000000000005', 'ffffffff-0000-7000-8000-000000000004')
ON CONFLICT DO NOTHING;

INSERT INTO attestations (id, tenant_id, user_id, regulation_id, regulation_version, text_hash, signed_at) VALUES
  ('eeeeeeee-0000-7000-8000-000000000001', '11111111-1111-7111-8111-111111111111',
   '44444444-0000-7000-8000-000000000001', 'ffffffff-0000-7000-8000-000000000004',
   1, 'sha256:conduct-v1-adainsignature', now() - interval '90 days'),
  ('eeeeeeee-0000-7000-8000-000000000002', '11111111-1111-7111-8111-111111111111',
   '44444444-0000-7000-8000-000000000007', 'ffffffff-0000-7000-8000-000000000004',
   1, 'sha256:conduct-v1-amakasignature', now() - interval '30 days')
ON CONFLICT (id) DO NOTHING;

COMMIT;

SELECT
  (SELECT count(*) FROM regulations)        AS regulations,
  (SELECT count(*) FROM course_regulations) AS course_links,
  (SELECT count(*) FROM attestations)       AS attestations;