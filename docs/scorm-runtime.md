# SCORM runtime (Phase 7 Part B)

Adds the SCORM 1.2 and SCORM 2004 run-time on top of migration 19's
`scorm_attempts` table: CMI data model, launch, commit, finish, and a
cookie-less API shim for the content iframe.

## Files

| File | Role |
| --- | --- |
| `apps/api/internal/scorm/cmi.go` | CMI keys, writable/readable sets, `InitialCMI`, `Derive`, session-time parsing, version detection. |
| `apps/api/internal/scorm/session.go` | HMAC-signed short-lived attempt token (`AttemptClaims`). |
| `apps/api/internal/scorm/runtime.go` | `Launch`, `Get`, `Commit`, `Finish`, `Abandon` against `scorm_attempts`. |
| `apps/api/internal/server/scorm_handlers.go` | HTTP routes + token middleware. |
| `apps/web/public/scorm/shim.js` | Content-side SCORM API. Exposes `window.API` / `window.API_1484_11`, mirrors CMI locally, relays via `postMessage`. |
| `apps/web/public/scorm/host.js` | Player-side host. Owns the attempt token and forwards commits/finishes to the API. |

## Wire it into the router

In your server setup, add:

    scormHandlers := server.NewHandlers(pool, []byte(cfg.SessionSecret))
    scormHandlers.Register(r)

`Launch` (`POST /v1/scorm/attempts`) sits behind the existing session
middleware, so `auth.FromContext(r.Context())` must be populated. The
remaining routes use the SCORM attempt token.

## Endpoints

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| POST | /v1/scorm/attempts | session | Open or resume an attempt. Body: `{ "lesson_id": "...", "version": "1.2" }`. |
| GET | /v1/scorm/attempts/{id} | Bearer token | Current CMI snapshot. |
| POST | /v1/scorm/attempts/{id}/commit | Bearer token | Persist a batch of CMI keys. |
| POST | /v1/scorm/attempts/{id}/finish | Bearer token | Commit plus completed_at and status transition. |
| POST | /v1/scorm/attempts/{id}/abandon | Bearer token | Marks an in-progress attempt abandoned. |

The token is HMAC-SHA256 signed with `SESSION_SECRET`. Default TTL is 4 hours.

## Browser flow

1. Player calls `POST /v1/scorm/attempts` with a session cookie.
2. Player receives `{ attempt_id, session_token, cmi, entry_point, package_id, version }`.
3. Player calls `VaultsideScorm.attach(iframe, { endpoint, token, version, cmi, contentUrl })`.
4. The content, on load, includes `shim.js`. It defines `window.API` /
   `window.API_1484_11`, reads its initial CMI from `window.name`, and keeps
   a local mirror so `GetValue`/`SetValue` are synchronous.
5. On `LMSCommit` / `Commit`, the shim posts the full mirror to the parent;
   the host POSTs it to the API with the attempt token.
6. On `LMSFinish` / `Terminate`, same plus the API stamps `completed_at`.
7. If the learner closes the tab mid-attempt, the shim posts `abandon`.

## Schema assumption

The launch query joins `scorm_packages ON scorm_packages.lesson_id = lessons.id`
and reads `scorm_packages.entry_point`. If your actual schema is the other
way around (`lessons.scorm_package_id`), change the join in
`runtime.go:Launch`.

## Not in Part B

- Certificate issuance on pass (hook at the handler).
- xAPI (Part C).
- Content unpacking / manifest parsing (worker side of Phase 2 upload pipeline).
- DB-backed integration test for Launch -> Commit -> Finish.

## Smoke test (no browser)

    curl -sX POST http://localhost:8080/v1/scorm/attempts \
      -H 'Content-Type: application/json' \
      -H 'X-Dev-Tenant: harbourline' -H 'X-Dev-User-Email: amaka@harbourline.example' \
      -d '{"lesson_id":"<lesson-uuid>","version":"1.2"}' | tee /tmp/launch.json

    ATTEMPT=$(jq -r .attempt_id /tmp/launch.json)
    TOKEN=$(jq -r .session_token /tmp/launch.json)

    curl -sX POST http://localhost:8080/v1/scorm/attempts/$ATTEMPT/commit \
      -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
      -d '{"cmi":{"cmi.core.lesson_status":"incomplete","cmi.core.score.raw":"45"}}'

    curl -sX POST http://localhost:8080/v1/scorm/attempts/$ATTEMPT/finish \
      -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
      -d '{"cmi":{"cmi.core.lesson_status":"passed","cmi.core.score.raw":"85","cmi.core.session_time":"00:12:34"}}'