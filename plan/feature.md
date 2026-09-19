# Feature and demo catalogue

2026-09-19. Target behavior is pending implementation. Existing Python features are reference material, not production completion.

## Required capabilities

| ID | Capability / observable outcome | Phase |
| --- | --- | --- |
| F01 | Import, preserve, validate and supersede official-style alerts/packages; no fabricated operational data | P2/P3 |
| F02 | Resolve spoken/typed locality using scoped aliases; clarify repeated names and uncertain ASR | P4/P6 |
| F03 | Show official zones and a small set of eligible immediate/temporary destinations | P4/P8 |
| F04 | Citizen chooses an allowed destination; no model allocation or compulsory default assignment | P4/P8 |
| F05 | Show verified mode-specific route plus landmarks; unavailable route yields honest fallback | P4/P8, activation P11 |
| F06 | Durable party reservation, arrival, cancellation, expiry, departure, transfer and stay extension | P3/P4 |
| F07 | Voice-first interaction with constrained middle model; useful optional TTS, no progress chatter | P6/P8 |
| F08 | Regional language content, place names, ASR and TTS validated for selected demo states | P6/P9 |
| F09 | Locally installed Android and iPhone UI; accessible chat/touch/non-map parity | P8/P9 |
| F10 | Verified downloaded maps/cards/audio, expiry/revocation and safe reconnect | P5/P8/P9 |
| F11 | Explicit OS dialler handoff; no automatic call/dispatch claim | P8/P9 |
| F12 | Operator publish/revoke/quarantine/source-health and jurisdiction-scoped corrections | P2–P4/P8 |
| F13 | Reconstruct source/version/policy behind every sensitive state transition | P3/P7 |
| F14 | Demonstrate loss of network/model/map/source and facility/route change without invented guidance | P7/P9 |
| F15 | Observable capacity, performance, privacy and security gates; reproducible deployment/restore | P7/P9/P10 |

## Proposed 15-state coverage

User confirmed 10–15 states and each state's regional language, but did not name the states. This is a proposal for scenario selection, not an approved event dataset. Each state needs 2–3 sourced historical flood/landslide cases in total (30–45 scenarios for 15 states). Narrow to 10–12 if evidence/zone availability warrants; do not silently lower language quality to reach a count.

| State | Relevant scenario focus to research | Primary service languages to validate |
| --- | --- | --- |
| Kerala | Flood, landslide | Malayalam |
| Tamil Nadu | River/urban flood | Tamil |
| Karnataka | Flood, Western Ghats landslide | Kannada |
| Andhra Pradesh | River/coastal flood | Telugu |
| Telangana | Urban/river flood | Telugu; assess Urdu need with user |
| Maharashtra | Flood, landslide | Marathi |
| Gujarat | River/urban flood | Gujarati |
| Rajasthan | Flash/river flood | Hindi; local dialect/place-name corpus |
| Madhya Pradesh | River flood | Hindi; local dialect/place-name corpus |
| Uttar Pradesh | River flood | Hindi; local-language needs by district |
| Bihar | River flood | Hindi; assess Bhojpuri/Maithili by district |
| West Bengal | Flood, hill landslide where relevant | Bengali; assess hill-language needs |
| Assam | Flood, landslide where relevant | Assamese; assess Bengali/Bodo by district |
| Uttarakhand | Flood, landslide | Hindi; Garhwali/Kumaoni place-name and speaker tests |
| Himachal Pradesh | Flood, landslide | Hindi; local Pahari varieties need evaluation |

English is an additional cross-state interface/voice candidate, not presumed part of the existing IndicConformer artifact. A state's primary language does not cover all its residents. Close the district/language matrix before claiming complete coverage; dialects and code-switching may need dedicated data or another speech model. Existing model weights are valuable, but “multilingual” is not an acceptance result.

## Scenario acceptance record

Every scenario has a stable `scenario_id`, state/district codes, historical event name/date, flood/landslide type, official report URL/title/access date, source limitations, event versus exercise time, and reviewer. Keep the historical event record distinct from `SYNTHETIC_EXERCISE` red zones, facilities, capacities and routes. User-provided demo geometry must carry ownership/reuse permission, CRS, IDs, timestamps and labels; if any are missing request them instead of guessing.

A complete exercise package includes origin/red zone, at least two meaningful destination choices where supported, mode-specific stored route candidates, temporary-stay properties, a clearly synthetic capacity/allocation policy and reviewed localized instructions. Do not invent a second destination merely to meet a quota. Demonstrate no-route, full facility, closure, same-name village, ambiguous speech, offline expiry and interrupted reservation. Vary hazards and terrain deliberately rather than copying one polygon to fifteen states.

Historical data supports an exercise narrative; it does not prove today's shelters, roads or capacity. Final event selection and demo zones are **awaiting user data / evidence**, not fetched or fabricated in this planning task. Keep the existing Wayanad exercise for regression; do not count it as a sourced historical case without provenance.

## Essential UI states

Ready to speak; recording/cancel; interpreting (quiet visual status); clarification; useful result; selectable destination preview; confirmation; reserved versus informational; route unavailable; full/closed facility; stale/conflicting source; offline valid cache; expired cache; pending reconnect; arrived/self-report; temporary stay ending; transfer pending; unsupported language; model/map unavailable; emergency contact.

Each state has one primary next action, readable text, screen-reader labels and a non-voice path. Large controls and predictable layout take priority over visual effects. Testing a child-friendly screen does not replace older-adult and disability-cohort testing.
