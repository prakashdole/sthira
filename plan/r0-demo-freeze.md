# R00 — Round-two demo contract freeze

R00 freeze commit SHA on `CLEAN`: see `git log -1 --grep '^R00:'` (the
"plan/r0-demo-freeze.md" added in that commit carries the SHA-less
reference on purpose so the doc itself never needs re-amending when
the SHA moves).

Owner: coordinator. Single reviewer/committer under the play's sequential
mode. No shared-file edits by other lanes until the SHA is committed.

## Scope of the freeze

The round-two demo exercises one coherent journey: speak → resolve place →
show labelled incident / destinations → choose → show supplied route →
useful audio. Stay and arrival are visible only when explicit. A distinct
second journey exercises the offline / errors path.

The freeze records:

1. Which `/api/v3` endpoints the demo calls and the request/response shape
   the UI must consume.
2. Which error codes are demo-visible vs internal.
3. Which minimal additional fields (if any) are introduced by the demo
   integration. None are introduced by R00 — every endpoint already exists.

The full contract remains `backend/contracts/openapi.yaml`. R00 freezes
*usage*, not the contract.

## Endpoint usage (demo)

All paths are prefixed with `/api/v3`; the UI does NOT call `/api/v2` —
the v2 prototype endpoints are reference-only for the old Python
`frontend/v1` client and are not exercised by the round-two UI.

| Step | Endpoint | Method | Notes |
| --- | --- | --- | --- |
| Boot: probe readiness | `/health/ready` | GET | 200 = dependencies + schema-migration-9 reachable. 503 BLOCKED is not a hard error. |
| Boot: session issuance | `/sessions` | POST | Account-free, returns bearer token once; the UI persists it in `sessionStorage` for the lifetime of the tab. No civil identity required. |
| Speak: full pipeline | `/voice/process` | POST | JSON body (typed `PipelineRequest`): base64 audio ≤ 512 KiB compressed, ≤ 20 s decoded, 8–48 kHz mono, plus session_id, language, jurisdiction, snapshot_version. Returns transcript, validated proposal, optional inline audio. |
| (alt) speak: text only | `/voice/transcriptions` | POST | Multipart/bytes audio, X-Language header. Returns typed transcript. UI uses this when model orchestrator is unavailable, then sends text to `/voice/commands`. |
| (alt) text direct | `/voice/commands` | POST | Already frozen by P1. Validates a typed middle-model proposal. |
| Resolve place | `/places/resolve` | POST | `jurisdiction` + `query`. AMBIGUOUS_PLACE 409 with candidate IDs; NOT_FOUND 404. No LLM geocode. |
| List eligible destinations | `/guidance/query` | POST | Read-only preview for a party and date range. Returns `EligibleChoice[]` (typed in `contracts/scoped.go`). Unknown capacity is reported, not promised. |
| Show supplied route | derived from `EligibleChoice.route` (GeoJSON LineString) | n/a | UI renders the supplied `route_id` geometry directly. No Google / public routing. |
| Synthesize approved speech | `/voice/speech` | POST | `jurisdiction` + `source_version` + `speech_key` + validated template args. Returns typed `TTSResponse` with content-addressed audio envelope. Source withdrawal during synthesis returns 409 STALE_SNAPSHOT. |
| Reserve a stay | `/reservations` | POST | `facility_id`, `package_id`, `route_id`, party_size, start_date, end_date, idempotency_key, snapshot_version. 201 Created on success; 409 STALE_VERSION/CAPACITY_CONFLICT on conflict; never silently substitutes a different destination. |
| Read own reservation (restart recovery) | `/reservations/{id}` | GET | Authenticated owner-only. Used after a lost response or browser restart. |
| Stay events | `/reservations/{id}/events` | POST | ARRIVE / CANCEL / DEPART / EXTEND / TRANSFER. EXTEND + TRANSFER require `snapshot_version > 0`; otherwise 400 INVALID_VALUE. ARRIVE requires explicit user confirmation (no automatic geofence). TRANSFER returns `new_stay_id` + `new_reservation_id` on first success and on idempotent replay. |
| Map data (offline / public) | `/regions/{id}/manifest`, `/packages/{id}/versions/{version}`, `/resources/{id}` | GET | | Public delivery with ETag / Range. Cacheable. UI uses these for offline / refresh, never raw public OSM bulk fetch. |

## Envelope (success + error)

Every response uses the standard `Envelope` from `contracts/openapi.yaml`:

- `request_id` (string): caller-supplied or server-generated correlation ID
  (the UI generates one per request and echoes it in errors/logs).
- `schema_version` (string, `"3.0"`): the contract version.
- `generated_at` (RFC 3339 UTC).
- `data_version` (string): the authoritative snapshot version the server
  resolved for the request. The UI must NOT echo client-cached
  `data_version` as authoritative; refresh via `/guidance/query` /
  `/reservations` / `/places/resolve`.
- `source_status` (one of `CURRENT, STALE, EXPIRED, UNAVAILABLE, UNKNOWN`):
  the server's view of freshness. UI surfaces this as a status pill
  (current demo wording already present in `frontend/v2/src/main.ts`).
- `data` is populated on success only; `errors[]` on failure only.

## Error codes the demo must handle (subset of the full taxonomy)

The full taxonomy is in `openapi.yaml` `APIError.code`. The demo UI must
explicitly handle:

| Code | HTTP | UI behavior |
| --- | --- | --- |
| `MALFORMED_JSON`, `DUPLICATE_KEY`, `TRAILING_DATA`, `BODY_TOO_LARGE`, `DEPTH_EXCEEDED`, `UNKNOWN_FIELD`, `UNSUPPORTED_MEDIA_TYPE`, `METHOD_NOT_ALLOWED` | 400 / 405 / 413 / 415 | Show as a client-side error; never retry with the same payload. |
| `INVALID_VALUE`, `VALIDATION_FAILED` | 400 | Show inline form errors. |
| `AMBIGUOUS_PLACE` | 409 | Render the candidate IDs as a clickable list; do NOT pick one automatically. |
| `STALE_VERSION` | 409 | Refetch `/guidance/query` and re-prompt the user to confirm against the new snapshot. |
| `ROUTE_UNVERIFIED` | 409 | The selected destination has no current route; offer the alternative destinations from the same `/guidance/query` response. |
| `CAPACITY_UNKNOWN` | 200 (not an error code, surfaced in `data.capacity`) | Show "capacity unknown"; never claim availability. |
| `CAPACITY_CONFLICT` | 409 | Show as a recoverable error; allow the user to pick another destination or wait. |
| `IDEMPOTENCY_CONFLICT` | 409 | Same-key / different-payload replay; show as a non-retryable error and require a new key. |
| `LANGUAGE_UNSUPPORTED` | 422 | Hide language switcher option; show as a notice in the chosen language. |
| `MODEL_UNAVAILABLE`, `TRANSCRIPT_UNAVAILABLE`, `AUDIO_UNAVAILABLE`, `MODEL_TIMEOUT`, `QUEUE_SATURATED`, `INFERENCE_CANCELLED`, `TEMPLATE_UNKNOWN`, `STALE_SNAPSHOT` | 503 / 504 / 499 | Voice path failure; UI keeps the typed chat input and the read-only map surface active (D58); no degraded safety claim is made. |
| `DATA_UNAVAILABLE` | 503 | `source_status = UNAVAILABLE`. UI shows "Offline / unable to reach guidance"; no map / safe-zone / route claims. |
| `FORBIDDEN` | 403 | For operations: means operator IdP is not wired; UI must not surface an operator button in this state. |
| `RATE_LIMITED` | 429 | Back off; do not retry within the same UI tick. |
| `NOT_FOUND` | 404 | Specific resource; clear the URL state and prompt re-selection. |
| `INTERNAL` | 500 | Server error; never assume success. |

## What the freeze does NOT add

- No new HTTP endpoint.
- No new request/response field.
- No new error code.
- No change to the envelope, the action enum, the language tags, the
  GeoJSON geometry types, the source status enum, or the snapshot
  semantics.

If the demo integration needs any of these, propose it via an amendment
that names the contract file, the section, the diff and the reason, and
do not edit the contract file unilaterally.

## Visible demo journey

(End-to-end run order on the demo laptop / phone. The exact UI copy lives
in `frontend/v2/src/i18n.ts`; the backend path is what this freeze owns.)

1. UI mounts. Fetches `/health/ready`. If 200 → green pill; if 503 →
   amber pill labelled BLOCKED (not an error). Reads `/api/v3/manifest`
   from a small list of `region_id` candidates for the chosen case.
2. UI posts `/api/v3/sessions`; persists bearer token to
   `sessionStorage` under `sthira.session.bearer`.
3. User taps the mic. UI records ≤ 20 s of mono audio, encodes it as
   `audio/webm;codecs=opus` (or `audio/wav` fallback), and posts
   `/api/v3/voice/process` with `language` matching the chosen UI
   language and `jurisdiction` matching the active scenario.
5. If `status == OK` and the proposal carries `intent = LIST_DESTINATIONS`
   or `intent = PREVIEW_DESTINATION`, UI calls
   `/api/v3/places/resolve` (if the proposal contains a place alias)
   then `/api/v3/guidance/query` with the resolved `package_id`,
   `party_size` and date range.
6. UI renders the `EligibleChoice[]` as labelled cards with the
   supplied `route` geometry on the map. The "Show route" button
   zooms to the supplied `route.bounds` only; no Google / public
   routing. If the proposal asks for speech, UI posts
   `/api/v3/voice/speech` and renders the returned audio.
7. On confirm, UI POSTs `/api/v3/reservations` with the chosen
   `facility_id`, `route_id`, party size, dates, a fresh
   `idempotency_key` (UUIDv4) and the confirmed `snapshot_version`.
   A 201 response is the success branch; a 409 STALE_VERSION forces
   a re-query and re-confirm; a 409 CAPACITY_CONFLICT forces a
   re-pick.
8. On the explicit arrival action, UI POSTs
   `/api/v3/reservations/{stay_id}/events` with `type = ARRIVE`. No
   `snapshot_version` is sent. UI shows the recorded-arrival message.

## Offline / error second journey

(End-to-end run when one or more dependencies fail. The same UI
screens are exercised; the backend surface is what changes.)

- Loss of model worker: `/health/ready` stays green (DB still
  reachable); `/voice/process` returns 503 MODEL_UNAVAILABLE. UI
  keeps the typed input + map; no safety claim is made; voice button
  is greyed with a "voice unavailable" notice. Text-only flow via
  `/voice/commands` continues to work.
- Loss of network: browser fires `offline`; UI status pill becomes
  amber `Offline`. The cached public manifest (if any) is shown
  with a `source_status = UNKNOWN` badge; the UI never claims an
  unchanged authoritative snapshot after restart (D55).
- Stale source withdrawal during a stay reservation: server returns
  409 STALE_SNAPSHOT; UI refetches `/guidance/query`, re-prompts the
  user with the new `snapshot_version` and rejects silent
  substitution.

## Pre-flight for the UI lane

The R03 lane may build the responsive UI against this freeze without
further contract changes. R03 must NOT introduce client-side
fabrication of:

- transcripts (the server returns the typed transcript)
- destination sets (the server returns `EligibleChoice[]`)
- route geometry (the server returns the supplied `route`)
- approved speech (the server returns audio only for `speech_key` in
  the typed template registry, after `approved_translations` lookup)

If R03 wants a new visible field, propose an amendment; do not edit
the contract file unilaterally.

## Pre-flight for the inference lane

The R02 lane may wire real private workers without further contract
changes. R02 MUST honour:

- the typed `WorkerHealth` / `ModelInfo` envelope shape
- the typed `TTSCacheKey` (source_version + template_version +
  model_revision + voice_revision + settings)
- the `STALE_SNAPSHOT` revalidation step that fires if a source is
  withdrawn between `BeginContext` and `Deliver`

If R02 needs a new typed field on `WorkerHealth` / `ModelInfo` /
`TTSCacheKey`, propose an amendment; do not edit the contract file
unilaterally.

## Pre-flight for the data lane

The R05 lane must produce one labelled synthetic exercise fixture (the
"R00 demo package") with a unique `package_id`, a unique
`jurisdiction` (e.g. `DEMO-R00-KL-WAYANAD`), an `evidence_class`
explicitly marked `SYNTHETIC_DEMO`, exactly one or two
`safe_zone` records, exactly one `approved_route`, and one or two
`facility` records so R01 has both the "one choice" and "two
facilities in one zone" cases.

R05 must NOT fabricate a 10–15-state catalogue for the demo.