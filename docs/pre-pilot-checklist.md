# Pre-pilot checklist

Items that must be verified before a paying tenant goes live. Each entry names
the artifact that satisfies it - either an automated test, a documented manual
check, or a configuration step.

## Phase 5 - Enterprise identity

- [ ] **Gate 5 - provisional.** Code is IdP-agnostic; Keycloak login verified
      end to end (OIDC and SAML). Needs verification against Microsoft Entra ID
      and Okta test tenants using docs/sso-setup.md.
- [ ] SCIM verified with a real IdP. Entra ID or Okta provisioning pushes Users
      and Groups into Vaultside; deprovisioning revokes active sessions.
- [ ] Multiple SSO connections for one tenant exercised (create a second
      connection, flip defaults, login via each).
- [ ] SSO admin UI reviewed by a non-engineer (currently API-only).

## Security

- [ ] Penetration test completed; no open high-severity findings.
- [ ] Dependency and container image scanning integrated into CI; no critical CVEs.
- [ ] Secrets rotation procedure documented (SSO_SECRET_KEY, SAML_KEY_B64,
      SESSION_SECRET).
- [ ] Password/SSO enforcement policy set per tenant (DEV_AUTH must be false in
      production).
- [ ] TLS 1.2+ enforced at the load balancer; HSTS headers set.
- [ ] Rate limiting per IP / user / tenant enabled and load-tested.
- [ ] Audit log retention meets tenant regulatory requirements.
- [ ] Data retention and deletion workflows tested (per-tenant, per-subject).

## Compliance

- [ ] SOC 2 Type I readiness review completed (access reviews, change
      management, incident runbooks in place).
- [ ] ISO 27001 gap assessment done; roadmap agreed.
- [ ] Data residency reviewed against the pilot tenant's jurisdiction
      (Nigeria, EU, etc.).
- [ ] Vendor contracts for AI / speech-to-text providers exclude training on
      tenant data.
- [ ] Regulator names, clause numbers, and jurisdiction-specific rules supplied
      by the tenant's compliance counsel - Vaultside leaves these configurable.

## Reliability

- [ ] Backup and restore drill passed; point-in-time recovery verified; time to
      restore recorded.
- [ ] Load test at 5,000 active learners per tenant: p95 under 300 ms for Today
      and docket.
- [ ] Observability wired: OpenTelemetry traces, structured logs with request
      and tenant IDs, dashboards for error rate, p95 latency, job queue depth.
- [ ] Runbooks for the top 5 incidents (DB failover, worker stall, expired
      certificate scan failure, storage quota, cache invalidation).

## Accessibility

- [ ] WCAG 2.2 AA audit by an external reviewer.
- [ ] axe in CI on every UI route.
- [ ] Manual keyboard-only pass on all primary flows (login, Today, quiz,
      manager docket, admin).
- [ ] Screen reader pass with NVDA and VoiceOver.
- [ ] Reduced-motion preference respected on every page.
- [ ] Accessibility statement published.

## Product and operations

- [ ] Pilot tenant seeded with their real departments, roles, users, and
      regulations.
- [ ] Content migrated or authored: at least one compliance course with an
      assessment, one onboarding path with a manager checklist.
- [ ] Compliance officer walkthrough completed; heat grid and audit pack
      reviewed by them.
- [ ] Learner walkthrough completed by 3 pilot users; blockers logged and
      resolved.
- [ ] Support channel established; escalation path documented.
- [ ] Incident response plan approved; on-call rotation set.
- [ ] Backup notification list for compliance and security alerts configured.

## Deferred to later phases

- SCORM 1.2 / 2004 / xAPI / cmi5 runtime
- AI guide with citations and transcript summaries
- Certificate PDF rendering with QR
- Theming beyond Expedition Atlas