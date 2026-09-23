# Phases and readiness gates

> **2026-09-21 priority override:** Round-two prototype/UI/real-model demo and pitch take priority through September 28–29. Read [round-two-demo.md](round-two-demo.md) first. Frontend prototype work may proceed before Gate B; production gates and the long-term roadmap remain intact.

Current execution entry point: [prompt.md](prompt.md), updated 2026-09-23. Its R/B/M/Q tasks prioritize the round-two demo, remaining backend work, P8 mobile delivery and P9 whole-system readiness; the P0–P12 roadmap below is retained. Go implementation exists; phase completion must be read from recorded evidence, not this original roadmap. Historical Python work is captured in [changes.md](changes.md).

| Phase | Deliverable | Verification gate |
| --- | --- | --- |
| P0 | Reconcile scope and freeze migration evidence | Actual paths/contracts/invariants and preserved baseline |
| P1 | Go foundation and executable contracts | Real HTTP/strict-schema/false-readiness tests |
| P2 | Government-data contracts and scenario ingestion | CAP/package/scenario evidence and malformed/replay tests |
| P3 | Durable storage, authorization and ledger foundation | Real PostgreSQL/PostGIS, authorization and multi-process durability |
| P4 | Destination choice and immediate/temporary stays | Choice and immediate/temporary capacity state-machine checks |
| P5 | Offline package and map-delivery protocol | Signed offline protocol, expiry/revocation and byte budgets |
| P6 | Regional ASR, constrained middle model and TTS | Real per-language ASR/LLM/TTS and serving benchmark |
| P7 | Backend security, performance and handoff gate B | Security/load/restore evidence; backend gate B |
| P8 | Select and implement Android and iPhone clients | Android+iPhone physical-device and accessible flow |
| P9 | Whole-system readiness, regional drills and release assurance | Regional drills, whole-system assurance and distribution |
| P10 | Retire obsolete files and verify the final artifact | Verified retirement and clean reproducible build; gate S |
| P11 | Authorized government integration and shadow exercises | Authorized live shadow/field evidence |
| P12 | Controlled launch and operational scale-out | Approved controlled pilot and measured expansion |

## Current playbook scope

`plan/prompt.md` now contains detailed execution tasks through P9: R-tasks for the demo, B-tasks through backend Gate B, M00–M05 for P8 and Q01–Q04 for P9. B05 is an intermediate checkpoint; Q04 is the final stop. Gate B still precedes P8 implementation. P9 requires actual both-platform, regional/language, security and recovery evidence; missing external approvals cannot be treated as a pass. P10 cleanup and P11/P12 operational activation/launch are not authorized by this extension.

## Sequencing

P0 → P1 → P2 contracts → P3 → P4/P5 → P6 → P7 (**B**) → P8 → P9 → P10 (**S**) → P11 → P12 (**L**).

P2's full scenario catalogue may await user zones, while contract-complete independent P3/P5 work proceeds. P7 does not declare B until backend acceptance is complete. P9 cannot pass without the selected state/case/language evidence. Do not use parallel activity to bypass unmet prerequisites or mark a partially complete phase DONE.

B defines the requested backend 80–90% milestone by capability, not line counts. S requires complete software, both mobile platforms, real model/device/language/security/load/recovery evidence and distribution readiness. L requires authorized government integration, route/capacity policy, field validation, operations and pilot approval. Until L, exercise modes stay unmistakably non-operational.

## Scope control

Production mobile framework selection remains P8. The user-authorized responsive round-two prototype may be implemented before Gate B, as described in the current playbook. Legacy deletion is P10 after replacements pass, with small earlier removals only if P0 proves no consumer/evidence dependency and the user authorizes that cleanup stage. No permanent-relocation modules are rebuilt. Existing inference frameworks need not become Go. Route authority remains open; there is no operational automatic-route-generation phase.

Each phase ends with tests, diff review, a verified local stage commit where safe, and a concise evidence/next-step update. Unknown APIs, absent devices/GPU, unsigned policies or missing scenario data are named blockers, not invented results.
