# Normative rules

Version 3 · 2026-09-19. Applies to the new plan; old hackathon-only instructions do not define the target product. Reconcile root agent documents in P0 before implementation; this planning pass does not edit them.

## Authority and input

- R01: Only authorized incident sources originate operational alerts, zones, facility suitability, instructions, closures, capacity and policy. Historical cases and demo geometry are visibly non-operational.
- R02: No hazard, red-zone, safe-land, route-safety or vulnerability prediction. Display transformations never acquire authority.
- R03: Source identity, jurisdiction, version, issue/effective/expiry time, retrieval time and validation state accompany operational facts. Missing, stale, conflicting or unapproved facts cannot authorize guidance.
- R04: CAP update/cancel references, package supersession and revocation invalidate dependent guidance. No resurrection from an older cache.
- R05: Government inputs, operator uploads, place names, audio and model output are untrusted data. Bound, validate and render them safely. A prompt is not a security boundary.
- R06: Public basemaps may provide visual context only after licensing/hosting approval. They do not establish incident passability. The old blanket prohibition on non-government basemap code/data is replaced by this distinction; operational truth remains government-authorized.

## Destinations, routes and capacity

- R07: The user may choose among eligible, policy-permitted destinations; the model cannot invent, rank by safety, allocate or override policy. Distance sorting requires known route lengths and a declared policy.
- R08: Route authority is OPEN. Until resolved, operational navigation is disabled; synthetic exercises and explicitly unverified map candidates stay isolated. An open decision is not permission to route citizens on guessed roads.
- R09: Never draw a straight line as evacuation guidance. Unknown access, surface, slope, bridge status or incident closure is not equivalent to passable. An approved evacuation may start inside a red zone; a naive “intersects red polygon” test is not sufficient to judge a route.
- R10: Every operational route is mode-specific, versioned, time-bounded and paired with landmarks/instructions. New closures invalidate affected guidance. If no verified alternative exists, disclose that and offer official help.
- R11: Explicit party size, capacity mode, reservation duration and stay dates are required where relevant. No implicit one-person or unlimited-capacity assumptions.
- R12: Capacity updates are durable ACID transactions, idempotent and jurisdiction-scoped. Free + held + occupied equals effective capacity when those categories apply; arrival converts held space to occupied without another decrement. Never clamp a negative result to hide overbooking.
- R13: Retries with the same scoped key and payload return the stored result; changed payloads conflict. Arrival, departure, cancellation, expiry, transfer and corrections must survive retries/restarts and be auditable.
- R14: Citizen arrival requires explicit visible confirmation; operator correction requires authorized scope. No geofence/voice-only capacity mutation. Self-reported arrival is labelled as such.
- R15: A temporary stay is 7–30 days within published facility/policy limits. Extensions, transfers and departures reconcile space; out-of-scope needs escalate without automated eviction.

## Models, accessibility and privacy

- R16: Middle-model output is schema-constrained and independently validated against the current request, source snapshot, jurisdiction and known IDs. No generated coordinates, executable code, URLs, SQL or arbitrary tools.
- R17: Model speech intent resolves to reviewed templates and source facts. No progress chatter. Useful results/clarifications/corrections remain speakable. Free-form generated emergency advice cannot reach TTS.
- R18: Unknown ASR confidence is unknown; do not substitute 1.0 or trust a self-reported LLM probability. Evaluate entity/intent accuracy and ambiguous place names per language.
- R19: Audio capture is explicit and bounded. No raw-audio retention/training by default; protect and expire transcripts/session context too. No full coordinates or secrets in ordinary logs.
- R20: Every voice/map function has readable chat/touch/screen-reader access. No model dependency for essential fallback. No tiny-only controls, color-only status or compulsory animation.
- R21: Only evaluated languages are enabled as supported. Model-card multilingual claims do not prove local place-name recognition or emergency translation quality. Approved critical wording is not freely translated by the middle model.
- R22: Public guidance needs no account or national identifier. Private reservations use unguessable, authenticated capabilities or sessions; knowing an assignment ID is not authorization. Operator identity is separate and strongly authenticated.
- R23: 112/local contact handoff requires an explicit user gesture and the platform dialler. No claim that a call connected or that help was dispatched without evidence.

## Offline, release and workflow

- R24: Bundled UI and valid downloaded data work without network. Unknown/expired status is visible. Offline writes remain pending until server acknowledgment; never announce a reserved bed from a queued request.
- R25: Signed packages have pinned trust roots, expiry, rollback prevention and explicit key rotation. A checksum only detects corruption; it does not authenticate a publisher.
- R26: Mobile clients, model workers and public API never receive government credentials. Source ingestion is separate, allow-listed, rate-limited and read-only by default. No arbitrary proxy endpoint.
- R27: Keep public read/cache traffic, sensitive writes, ingestion and model inference isolated by credentials and workload limits. Retry only when safe; bound queues and shed optional work before emergency data.
- R28: Demo, test, shadow and production use distinct data, keys, notification targets and visible mode labels. Production builds do not contain demo routes or legacy endpoints.
- R29: A readiness endpoint proves actual dependencies and approved source state; environment-variable presence alone is not readiness. Liveness is separate.
- R30: Pass measured functionality, safety, security, privacy, accessibility, language, load and restore gates before release. No tool guarantees absence of defects.
- R31: Preserve existing work and all `.txt` files. This task edits only `plan/*.md`. Later cleanup follows [cleanup.md](cleanup.md), retains behavior evidence, and never deletes source just to reduce line count.
- R32: Commit verified stages only when their changes can be separated from pre-existing work. Do not push, deploy or test attacks on external/government systems without explicit authorization.
