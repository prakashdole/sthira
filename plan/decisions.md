# Decision record

2026-09-19. **Accepted** means explicitly chosen by the user or retained as a governing safety boundary. **Proposed** means an implementation recommendation awaiting its stated gate. Superseded details remain in Git rather than the active backlog.

| ID | Status | Decision and reason | Revisit / evidence |
| --- | --- | --- | --- |
| D01 | Accepted | Immediate evacuation and 7–30 day temporary stays; no permanent relocation or safe-land/hazard prediction | User pivot, 2026-09-19 |
| D02 | Accepted | Go owns product backend; migrate required behavior rather than every legacy module | User pivot; P0 inventory |
| D03 | Accepted | Android and iPhone available from launch; defer frontend framework until backend handoff P7 | User clarification |
| D04 | Accepted | One million total users; benchmark explicit incident concurrency scenarios | User clarification; sizing assumptions P7 |
| D05 | Accepted | 10–15 demo states, 2–3 flood/landslide cases per state, regional languages; user supplies zones later | State list proposed in feature.md |
| D06 | Accepted | ASR → middle language model → TTS with voice-first UX and chat/touch alternatives | User pivot; no unnecessary spoken acknowledgments |
| D07 | Accepted | Government owns operational zones, facilities, capacity and instructions. Model cannot generate operational facts | User pivot and retained authority boundary |
| D08 | Open | Route approval/verification ownership is not yet selected | User explicitly left open; O05 |
| D09 | Accepted | User chooses among eligible destinations and route options where policy allows | User's local-knowledge requirement; no unverified route endorsement |
| D10 | Accepted | Local UI/assets/maps/cache; server AI and authoritative dynamic data | User requirement; offline freshness still enforced |
| D11 | Proposed | Use isolated established Python/native inference runtimes while product logic moves to Go | Avoid porting vLLM/ASR/TTS frameworks; P6 compatibility record |
| D12 | Proposed | Modular Go service, PostgreSQL/PostGIS, SQL transactions, HTTP/CDN first; no compulsory broker/Redis/Kubernetes | Smallest operational architecture; expand only on evidence |
| D13 | Proposed | Benchmark Qwen3-4B-Instruct-2507 first with private vLLM; final model selected on regional intent/entity performance | P6 corpus, license/version/hardware evaluation |
| D14 | Proposed | Retain IndicConformer and Indic Parler-TTS as first candidates, not blanket verified language support | P6 exact artifact/license and per-language evidence |
| D15 | Proposed | MapLibre Native with licensed downloadable vector regions; remove dependence on raster imagery | Both-platform offline/device tests P8; map license O06 |
| D16 | Retained | Explicit arrival and dialler confirmation; no geofence, automatic calling or model capacity writes | Existing safety boundary; conservation tests |
| D17 | Retained | Durable atomic idempotent capacity, authoritative versions and invalidation | P3/P4/P7 database/failure evidence |
| D18 | Accepted | This task changes plan Markdown only. Actual cleanup and Go implementation are later phases | User's final scope/cost instruction |
| D19 | Proposed | Layered OSS assurance: Go native checks, Staticcheck/pprof/k6, vulnerability/secret scans, ZAP plus scoped Strix | P7/P9; Strix is not a security certificate |
| D20 | Accepted | Frontend starts at measurable backend gate B, not a claimed 80–90% based on LOC | Operational definition of requested sequence |
| D21 | Accepted | Go 1.27.1 pinned as the backend toolchain; stdlib-only P1 slice builds offline | P1; revisit on supported-release cadence |
| D22 | Accepted | `/api/v3` contract slice frozen in `backend/contracts/openapi.yaml`; v2 kept only as reference for the old demo client during migration | P1; P10 retires v2 compatibility |
| D23 | Accepted | Strict JSON boundary rejects duplicate keys, unknown fields, trailing data, oversized bodies and excess depth before typed decoding | P1; R05 trust boundary |
| D24 | Accepted | Citizen sessions are account-free capability tokens: crypto/rand bearer, only SHA-256 hash stored, expiry + revocation; knowing an ID is not authorization (R22) | P4; no mandatory civil identity |
| D25 | Superseded | Operator authority requires verified identity + MFA (sessions.mfa_verified_at), not the OPERATOR label alone; operations jurisdiction-scoped, cross-jurisdiction FORBIDDEN | P4; R22; SUPERSEDED by D29 — the original slice trusted a caller-supplied request-body `mfa_verified:true` flag and caller-chosen jurisdiction, which is self-attestation, not verified identity or MFA |
| D29 | Accepted | Operator issuance requires a server-verified identity + MFA from a trusted authentication boundary (OperatorVerifier seam); jurisdiction/privilege derive from server-controlled operator_grants bound to the verified subject, never from the request; fails closed (503) with no verifier wired; synthetic verifier only in isolated tests, never in the production binary. Live operator IdP is BLOCKED_EXTERNAL (no IdP in deps) — recorded in open-decisions, NOT deferred to P11 | P4; R22; corrects D25 |
| D30 | Accepted | Operator idempotency is resource-bound: the target ID is folded into the operation name (source.transition:<id>, source.quarantine:<id>, stay.correction:<id>) over the existing IdempotencyStore, and current authorization for the target is revalidated before any replay result is disclosed; same-key/different-target is a distinct resource-scoped key, never success for the wrong target. Compatibility: the unique key is (scope, operation, idem_key); pre-change completed rows used bare operation names, so they occupy a different key space and are never wrongly replayed under the new target-scoped names — old rows are inert, no migration needed | P4; idempotency integrity |
| D31 | Accepted | Quarantine is a terminal sourceact QUARANTINED state distinct from SUSPENDED, reachable from all active states; quarantined evidence is excluded from the resolved operational context and new-reservation revalidation; existing stays are not cancelled nor capacity released | P4; source lifecycle |
| D32 | Accepted | The voice-commands context resolver is persisted: store.ResolveContext/ResolveAnyOperationalContext derive data version, known IDs and languages from one consistent current OPERATIONAL+authorized package row, failing closed on absent/expired/unauthorized/quarantined evidence; the static demo resolver stays explicitly non-operational | P4; no client-echoed facts |
| D26 | Accepted | Stay dates are half-open [start, end) in facility-local-date semantics (facilities.timezone); capacity E = free+held+occupied conserved per service_date; arrival converts held->occupied with no second decrement | P4; equations.md |
| D27 | Accepted | Operator stay correction is downward-only (releases space, always conservation-safe); an increase needs new capacity and is rejected rather than silently overbooking | P4; capacity conservation |
| D28 | Accepted | Multi-date reservations/transfers lock inventory in deterministic (facility_id, service_date) sorted order to avoid deadlock | P4; concurrency evidence |

## Material changes from the previous plan

- Old deterministic-only language parsing becomes constrained language interpretation plus deterministic action/domain validation. Retain the deterministic fallback for simple known commands.
- Old preassigned single-destination demo becomes citizen selection among eligible options; authority policy can still constrain choices.
- Wayanad-only pilot content expands to a proposed multi-state historical exercise matrix. Live coverage is separately authorized.
- Native apps are now launch scope; the web demo remains a migration/reference client.
- Government-only **operational truth** remains mandatory. A licensed public/self-hosted basemap is a possible visual dependency, not a new authority source.
- The Python-phase DONE labels are historical evidence, not proof that new Go phases have passed.
- Model sizes, server counts, performance budgets and third-party licenses are not silently accepted because a plan mentions them.

## Recording later decisions

For a change, record owner/date, problem, chosen option, alternatives, concrete evidence, affected files/contracts, consequences and rollback/revisit trigger. Do not close an open route/source/capacity policy without the responsible authority's evidence. Never recycle an old decision ID to conceal a changed meaning.
