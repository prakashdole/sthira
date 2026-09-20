# Open decisions and external dependencies

Updated 2026-09-19. Owner names are roles until an actual person is assigned; no approval is implied. Closed user clarifications: total users = one million; Android+iPhone launch; 10–15 states with regional languages; route authority deliberately open.

| ID | Unresolved decision / evidence needed | Owner | Blocks |
| --- | --- | --- | --- |
| O01 | Approved demo state/district list, 2–3 historical cases each, official references and user-supplied zones/routes | Product/user + scenario curator | Full P2/P9 scenario acceptance; fixture infrastructure can proceed |
| O02 | Minimum Android/iOS versions and named physical 3 GB Android/lower-end iPhone test devices | Product/mobile | P8 device budget acceptance |
| O03 | Regional service-language/dialect matrix including English, ASR/TTS artifact licenses and human reviewers | Product/voice/accessibility | P6/P9 full language acceptance |
| O04 | Monthly normal/surge hosting budget, provider, available GPU, India residency and on-call operator | Product/operations/security | Final P7 sizing and gate S; local code can proceed |
| O05 | Who approves routes, how local verification is recorded, mode/closure freshness and conflict escalation | Government/local response authority | All operational navigation; explicitly OPEN per user |
| O06 | Basemap/gazetteer/elevation licenses, offline rights, hosting/attribution and redistributable map pack format | Maps/legal/operations | P5/P8 production maps |
| O07 | Facility inventory, capacity mode, reservations/expiry, walk-ins, stay dates, extensions/departures/transfers and authority policy | Responsible shelter authority | Live assignment/stay activation; test policies remain synthetic |
| O08 | Source agreements, documented endpoints/auth/quota/schema/coverage, samples, publisher keys and incident instruction precedence | NDMA/SDMA/DDMA liaison | P11 source activation |
| O09 | Accepted peak traffic mix, availability/restore budgets, regional failure model, infrastructure funding and staffing | Operations/product | Final backend/whole-system assurance |
| O10 | Session recovery, child/caregiver use, minimum location precision, transcript/session retention and deletion | Product/privacy/security | P8/P9 launch privacy review |
| O11 | Approved instruction translations/ISL corpus and qualified review partners | Content/accessibility/authority | Language/ISL claims and final gate S |
| O12 | Notification transport and permission/distribution plan for Android/iOS; background limits and delivery ownership | Mobile/operations | P8/P9 alert delivery; no guaranteed push claim |
| O13 | Frontend framework, map/audio/storage integrations and maintainers for both platforms | Mobile/product | P8 decision, intentionally deferred until B |
| O14 | Operator authentication provider, role matrix, publisher/correction workflow and emergency access SOP. P4 delivered the engineering seam: a trusted-boundary OperatorVerifier (server-verified identity + MFA), server-controlled operator_grants binding verified subject to jurisdiction, and a fail-closed default (issuance 503, no verifier wired in production). BLOCKED_EXTERNAL on choosing the real IdP/MFA provider — no IdP exists in the current dependency set. Until resolved, live operator authentication stays disabled and is NOT claimed complete; this is an operational/external gap, not unfinished engineering, and is not silently deferred to P11 | Security/operations/authority | Live operator issuance; the trusted IdP/MFA boundary integration |
| O15 | Strix exact pinned version/model, spending limit and staging attack scope; independent human security reviewer | Security/product | P7/P9 active security exercise |
| O16 | App-store accounts/signing, release/update distribution, supported device support window and incident communication | Release/mobile | Gate S software distribution readiness |

## Policy for unresolved items

Do independent engineering with explicit synthetic fixtures and conservative non-operational states. Do not mark blocked evidence DONE, invent an authority policy, or convert a proposal into a production default. Real hardware/model testing, security remediation, usability and operations are internal readiness work even when procurement is needed; they cannot be relabelled “government APIs remaining.”

O05 does not prevent building a schema, importer, synthetic route viewer or validation tests. It does prevent promising a real evacuation route. Similarly, missing user zones need not block a generic fixture validator but does block claiming the 30–45-case catalogue is complete.
