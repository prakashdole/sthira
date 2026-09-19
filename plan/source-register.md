# Data, model and technical source register

Updated 2026-09-19. Public documentation is evidence about a product, not permission or proof of an operational feed. No live government endpoint, credential, quota or multi-state coverage was verified during this planning task.

## Operational sources to qualify

| ID | Source / discovery link | Required purpose | Current treatment |
| --- | --- | --- | --- |
| G01 | [NDMA SACHET](https://sachet.ndma.gov.in/) and [CAP 1.2 standard](https://docs.oasis-open.org/emergency/cap/v1.2/CAP-v1.2.html) | Alerts, official severity/instructions, update/cancel | Candidate backbone; documented access/cache/quota/coverage still required |
| G02 | [IMD](https://api.imd.gov.in/) | Official rainfall/weather context where needed | Context only, no locally derived hazard prediction; scope exact products |
| G03 | [CWC](https://ffs.india-water.gov.in/), [NWIC portal](https://nwdp.nwic.gov.in/) | Flood observations/forecasts and station coverage | Authorized sample/schema/freshness required; portal is not a presumed API |
| G04 | [GSI Bhusanket](https://bhusanket.gsi.gov.in/) | Official landslide bulletins/inventory | Susceptibility/forecast is not an evacuation route or shelter certificate |
| G05 | Relevant SDMA/DDMA/local bodies, including [KSDMA](https://sdma.kerala.gov.in/) and [Wayanad](https://wayanad.gov.in/) reference work | Incident zones, facilities, stay/capacity policy, instructions, operations | Replace/extend jurisdiction by jurisdiction; no national uniform contract presumed |
| G06 | Responsible roads/local response authority | Verified routes, bridge/path access, closures, landmarks and modes | Owner/approval workflow explicitly OPEN (O05) |
| G07 | Government/legally reusable administrative gazetteer | Stable state/district/locality IDs and multilingual aliases | License, coverage and same-name resolution required |
| G08 | [ERSS 112](https://www.112.gov.in/) and official local directories | Explicit phone dialler handoff | No dispatch API or guaranteed connection implied |
| G09 | [NDEM](https://ndem.nrsc.gov.in/), [Bhuvan](https://bhuvan.nrsc.gov.in/) if needed | Licensed government geospatial context | Optional; authorization/offline/redistribution terms required |

Do not add unrelated earthquake/fire/ocean/land-acquisition connectors just because older plans listed them. Current demo scope is floods and landslides. Historical official reports are curated as scenario evidence; demo route/shelter geometry remains a separate synthetic source class.

## Activation and source security

`DISCOVERED → ACCESS_REQUESTED → SAMPLE_ACQUIRED → VALIDATED → AUTHORIZED → OPERATIONAL`, with suspension/retirement at any relevant point. A configured URL is not activation. For each source record owner/contact, legal/reuse basis, auth/secret reference, fixed hosts/redirect policy, quota/cache rules, jurisdiction, hazard/language coverage, schema/units/CRS/null semantics, timestamps/expiry, update/cancel behavior, raw sample digest, validation results, conflict policy, retention, escalation and named sign-off.

Source adapters fetch once per agreed schedule, not once per app user. Store raw input restricted; publish only validated projections. Suspended/stale sources cannot be laundered through a cached package as current. Precedence between agencies must be approved per incident; do not hard-code “safer” guesses or assume every DDMA document overrides every central alert.

## Primary technical evidence consulted for this revision

| ID | Reference | What was verified / limitation |
| --- | --- | --- |
| S01 | [Qwen3-4B-Instruct-2507 model card](https://huggingface.co/Qwen/Qwen3-4B-Instruct-2507) | Official README retrieved 2026-09-19: 4.0B parameters, Apache-2.0 label, non-thinking model. No Sthira regional benchmark performed |
| S02 | [vLLM structured outputs](https://docs.vllm.ai/en/latest/features/structured_outputs/) | Documentation retrieved: JSON schema/structured generation supported. Pin runtime; validate semantics independently; no throughput claim |
| S03 | [MapLibre Native Android OfflineManager](https://maplibre.org/maplibre-native/android/api/-map-libre%20-native%20-android/org.maplibre.android.offline/-offline-manager/index.html) | Offline-region API documentation retrieved. Exact iOS/pack-format behavior must be verified at P8 |
| S04 | [OSMF tile usage policy](https://operations.osmfoundation.org/policies/tiles/) | Retrieved policy prohibits public raster tile bulk/offline download; recommends self-hosting or a provider allowing it |
| S05 | [Google Map Tiles policy](https://developers.google.com/maps/documentation/tile/policies) | Retrieved policy restricts prefetch/cache/offline use; specific product terms apply, not a latency benchmark or a blanket statement about all Maps SDKs |
| S06 | [Strix repository](https://github.com/usestrix/strix) | Official README retrieved: Apache-2.0 badge, Docker/local open-source workflow using an LLM. No scan run; pin/review actual release/license before use |
| S07 | [Go security](https://go.dev/doc/security/) | Official documentation retrieved for govulncheck and security tooling |
| S08 | [Staticcheck](https://staticcheck.dev/docs/) | Official documentation retrieved: static Go bug/performance/style checks; free/open-source claim; no application test run |
| S09 | [Go profiling](https://go.dev/blog/pprof) | Official profiling guidance retrieved; supports measuring bottlenecks instead of inferring from language |
| S10 | [ZAP API documentation](https://www.zaproxy.org/docs/desktop/start/features/api/) | Official API documentation retrieved; proposed dynamic-testing tool, not run here |

Retrieval used direct primary READMEs/pages and Jina-rendered documentation. Firecrawl's configured credentials failed; no credentials/configuration were changed. These are documentation checks, not live runtime or performance validation. Save exact release/model hashes and license text at implementation, since documentation can change.

## References requiring implementation-time verification

- Existing [IndicConformer](https://huggingface.co/ai4bharat/indic-conformer-600m-multilingual) and [Indic Parler-TTS](https://huggingface.co/ai4bharat/indic-parler-tts) artifacts are retained candidates. Direct ASR model-card access was authorization-gated in this pass. Verify downloaded artifact/license/language tables without assuming earlier “22 languages” claims establish enabled app support. Current code explicitly enables only Hindi/Malayalam ASR.
- [Valhalla](https://github.com/valhalla/valhalla) / [GraphHopper](https://github.com/graphhopper/graphhopper): candidate-route preparation only if O05 requires it. Valhalla documentation fetches did not establish usable API details; exact routing/exclusion/surface features are unverified here.
- [OpenStreetMap copyright/ODbL](https://www.openstreetmap.org/copyright): review data and produced-database/attribution obligations with the chosen tile source. Free data is not free hosting.
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/), [GIGW](https://guidelines.india.gov.in/), [ISLRTC](https://islrtc.nic.in/): qualified human accessibility/emergency-language review remains required. Dictionaries are not approved emergency scripts.
- Other tool links in [assurance.md](assurance.md) identify candidates; exact versions, licenses and scanner support must be recorded before adoption. No “industry standard” label substitutes for threat coverage.
