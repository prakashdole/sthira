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
