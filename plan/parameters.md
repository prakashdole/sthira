# Parameter and budget registry

2026-09-19. **Proposed engineering targets, not measured results or government policy.** P0/P7 freeze test profiles; changes need evidence and a decision record. Exact provider URLs, quotas, route validity and capacity policy cannot be invented.

## Accepted scope and proposed resource budgets

| Parameter | Value / treatment | Owner / gate |
| --- | --- | --- |
| Total-user planning target | 1,000,000 | User confirmed |
| Simultaneously active sessions | 10K baseline; 50K surge; 100K stress, assumptions | Operations, P7/P9 |
| Normal read rate per active user | 0.1 requests/s before cache, synthetic assumption | Load model |
| Edge/device cache hit assumption | 95% for public reads only; test cold-cache loss too | Load model |
| Write arrival rate | 0.002 writes/s per active user, assumption | Load model |
| Voice arrival rate | 0.002 utterances/s per active user, assumption; test 5× surge | Inference load model |
| Android memory class | Physical device with 3 GB total RAM; app budget is smaller | User requirement, P8/P9 |
| iPhone baseline | Lower-end supported model + minimum iOS TBD | Product, O02 |
| App download excluding optional packs | Initial target ≤40 MiB per platform/device variant | P8 measurement; revise by ADR |
| App memory | Initial p95 resident/PSS-equivalent target ≤300 MiB steady, ≤450 MiB map peak | Device tests; OS metric recorded |
| Cold offline useful screen | p95 ≤3 seconds with a valid downloaded card | P9 |
| Foreground cached interaction | p95 ≤250 ms for non-map actions | P9 |
| Critical initial incident card | ≤64 KiB compressed; map/audio not on critical path | P5 |
| Optional district map pack | Aim ≤50 MiB for the declared area/zoom/style; show size before download | P5/P8; not all India |
| Normal constrained network test | 400 kbit/s down, 128 up, 400 ms RTT, 2% loss | P5/P9 |
| Degraded network test | 128 kbit/s down, 64 up, 800 ms RTT, 5% loss; complete outage also | P9 |
| Guidance API processing | p95 ≤300 ms, p99 ≤1 s at accepted origin load, excluding network | P7 |
| Capacity write processing | p95 ≤1 s at accepted hotspot load; correctness mandatory | P7 |
| Voice after speech ends | Initial p95 ≤6 s to useful action on normal constrained network | P6/P9; measure all stages |
| Availability | Proposed 99.95% monthly critical API SLO; third-party freshness reported separately | Operations agreement before S |
| Durable writes | No acknowledged write loss under the tested supported single-node failure; DR RPO/RTO separately approved | P7/P9 |
| Disaster recovery | Initial RPO ≤5 min for regional disaster, RTO ≤30 min; rehearse and approve | Operations, O09 |

A stress tier is not a traffic forecast. User population does not determine request rate by itself. Freeze traffic mix, package sizes, geography skew, voice proportion, cold-cache behavior and error injection with every result. On battery/thermal tests record duration, radio/network and screen brightness; select a measured battery-drain budget at P8 rather than claiming one now.

## Voice/resource limits — proposed defaults

| Parameter | Initial candidate | Constraint |
| --- | --- | --- |
| Recording duration | Typical 5–10 s, cap 20 s | Permit stop/retry; no always-on listening |
| Upload | Negotiated compressed mono audio; cap 512 KiB compressed plus decoded-duration/sample bounds | Validate codec contents, not client headers |
| Middle-model context | 2–4K tokens; max 4K initially | Retrieve candidate subset; no whole-state catalogue |
| Generated output | ≤256 tokens, ≤5 actions | Typed schema + bounded counts |
| Inference deadline | Proposed 8 s total server processing cap | Cancellation and short fallback; tune from P6 |
| Raw audio retention | Zero after processing by default | Also cover crash/temp files and worker logs |
| Session text/location retention | Minimum operational duration; exact TTL open | Privacy owner; no indefinite conversation history |
| LLM temperature | Deterministic/low-variance setting supported by pinned runtime | Not a safety guarantee |
| ASR threshold per language | No fabricated universal threshold | Measured entity/intent gate; unknown confidence not 1.0 |
| Voice safety eval | Zero accepted invented IDs/prohibited mutations in adversarial corpus | Finite test guarantee only |
| Language task success | Proposed ≥95% critical-intent/entity success per language/cohort, with uncertainty reporting | Human review, P6/P9 |

## Authority-owned configuration — no production defaults

`source_max_age`, polling/quota, source/issuer allow-list, CRS, incident conflict precedence, route approval owner, verification/closure freshness, facility status, mode/accessibility constraints, capacity mode, reservation TTL, temporary-stay duration/extension/transfer policy, instruction translations, emergency directory, retention and authorized publisher keys.

Every value includes owner, jurisdiction, evidence, effective time, version and review trigger. Missing required values disable the dependent action. Synthetic test policies are explicitly identified as test policy and never promoted as real policy. Cache validity cannot exceed the earliest applicable source/package/policy expiry.
