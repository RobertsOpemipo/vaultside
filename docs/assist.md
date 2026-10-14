# AI assist (Phase 7 Part D)

Provider interface, ask endpoint with citations, and the per-tenant flag
that gates the side panel.

## Files

| File | Role |
| --- | --- |
| `apps/api/internal/assist/provider.go` | `Provider` interface, `Noop` default, `New(name)` selector. |
| `apps/api/internal/assist/service.go` | Retrieval + citation building. |
| `apps/api/internal/server/assist_handlers.go` | HTTP routes + flag check + permission check. |
| `apps/api/db/migrations/00020_assist_flag.sql` | `tenants.assist_enabled boolean` (default false). |

## Endpoints

| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/v1/assist/status` | session | `{enabled, provider, model}`. Frontend uses `enabled` to decide whether to show the panel. |
| POST | `/v1/assist/ask` | session | Body `{lesson_id, question}`. 403 if the flag is off. Returns `{answer, citations, provider, model}`. Writes to `audit_log`. |
| GET | `/v1/admin/assist` | session + `tenant.manage` | Current flag. |
| PATCH | `/v1/admin/assist` | session + `tenant.manage` | Body `{enabled: bool}`. Writes to `audit_log`. |

## Provider

Only `noop` ships today. `assist.New("anthropic")` returns an error and the
handler falls back to noop with a warning log. Adding a real provider means
writing one file that satisfies `assist.Provider` and adding a case to
`assist.New`.

## Guardrails (what's already enforced)

- Per-tenant flag, default off.
- Only `tenant.manage` holders can flip it.
- Every ask and every toggle writes to `audit_log`.
- `assist.ask` audit rows include the question, the retrieved citations, and
  the provider name — so a compliance officer can see what was asked and
  what the system used to answer it.
- The noop provider never leaves the process; no network call.

## What Part D does *not* do

- Real model integration. Provider is behind an interface; drop one in
  later with no handler changes.
- Real retrieval. The noop path only fetches the lesson title as context.
  Real retrieval (lesson body, transcripts, published course content) is
  the next thing to build when a real provider lands.
- Transcription. The `transcripts` table exists from migration 19; the
  transcribe worker is deferred.