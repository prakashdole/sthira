---
title: Sthira Government Data Source Register
document_id: STHIRA-SOURCES
version: 2.0
status: Researched candidate register; access must be verified before use
as_of: 2026-09-11
---

# Government Data Source Register

## 1. Policy

Production operational facts come only from Indian Union, State, UT, district, or local-government authorities and their formally authorized systems. An official web page proves discoverability, not API entitlement, freshness, pilot coverage, or permission to republish. Each connector must pass the activation gate in §5.

Open-source software and AI4Bharat models are dependencies, not disaster-data sources. No private weather, maps, routing, location enrichment, analytics, or emergency feed is permitted in the production emergency path.

## 2. Priority integration sources

| ID | Authority/product | Official access | Product use | Qualification |
| --- | --- | --- | --- | --- |
| GOV-001 | [NDMA SACHET](https://sachet.ndma.gov.in/) | CAP portal, [RSS/CAP feed](https://sachet.ndma.gov.in/CapFeed), [agency integration guide](https://sachet.ndma.gov.in/docs/Integration_Guide_For_Agencies.pdf) | Primary cross-hazard alert, area, severity, instruction, update/cancel | Implement ETag caching; confirm feed terms and production polling agreement |
| GOV-002 | [NDMA SACHET project description](https://ndma.gov.in/Capacity_Building/Ops_Comm/IT_Comm_Project) | Public programme information | Authority chain and alert-generating agencies | Descriptive, not a data endpoint |
| GOV-003 | [IMD API platform](https://api.imd.gov.in/) and [reference](https://api.imd.gov.in/public/api_reference.html) | APIs; some access may require IP whitelisting | District/station nowcast, warnings, rainfall, AWS/ARG, cyclone and related weather context | Preserve units, issue/valid times, warning colour, attribution, and coverage |
| GOV-004 | [IMD Mausam APIs page](https://mausam.imd.gov.in/responsive/apis.php) | Documentation and access instructions | Connector registration and operational contact | Confirm production terms and rate limits |
| GOV-005 | [CWC flood forecasting](https://ffs.india-water.gov.in/) | Public portal; agency/API arrangement as available | Official river flood levels and forecasts | Station coverage is not universal; do not generalize beyond station/basin |
| GOV-006 | [CWC 7-day advisory](https://aff.india-water.gov.in/) | Public portal/agency feed | Longer-range official flood advisory | Label advisory horizon and uncertainty |
| GOV-007 | [National Water Data Portal — CWC](https://nwdp.nwic.gov.in/organization/cwc) | Dataset download/API where listed | River level, discharge, rainfall and station metadata | Verify dataset update time and API credentials |
| GOV-008 | [GSI Bhusanket](https://bhusanket.gsi.gov.in/) | Public bulletins/maps/downloads; governed feed if agreed | Official landslide forecast bulletins, susceptibility/inventory context | Susceptibility is not an evacuation order or site-safety declaration |
| GOV-009 | [ISRO/NRSC NDEM](https://ndem.nrsc.gov.in/) | Authorized portal/export | Government disaster geospatial layers | Access and redistribution require explicit authorization |
| GOV-010 | [ISRO Bhuvan WMS/WMTS](https://bhuvan.nrsc.gov.in/wiki/index.php/How_to_use_WMS_services) | OGC web services | Government basemap/thematic display where licensed | WMS imagery is display data, not routing or safe-zone authority |
| GOV-011 | [INCOIS](https://incois.gov.in/) | Official ocean information and warning services | Tsunami, storm surge, ocean-state advisories | Obtain documented machine feed/coverage before connector activation |
| GOV-012 | [Forest Survey of India](https://fsi.nic.in/) | Official forest-fire products/alerts | Forest-fire alert enrichment where available | Prefer SACHET alert as citizen alert backbone |
| GOV-013 | [National Center for Seismology](https://seismo.gov.in/) | Official earthquake information | Earthquake event context | Event data does not by itself provide evacuation routing |
| GOV-014 | [DGRE](https://drdo.gov.in/drdo/labs-and-establishments/defence-geoinformatics-research-establishment-dgre) | Official avalanche products through authorized channels/SACHET | Avalanche warning where covered | Confirm public feed and permitted dissemination |
| GOV-015 | [Open Government Data Platform India](https://www.data.gov.in/) | Catalog downloads and APIs | Supporting official reference datasets | Catalog entries vary in freshness, license, granularity, and operational fitness |
| GOV-016 | [ERSS 112](https://www.112.gov.in/) and [MHA ERSS](https://www.mha.gov.in/en/divisionofmha/women-safety-division/emergency-response-support-system-erss) | Device telephone call; official app/process | Citizen-initiated emergency call | Sthira opens the dialler; it is not an ERSS dispatch integration |

## 3. Pilot/state operational sources

| ID | Authority/product | Intended data | Required acquisition route |
| --- | --- | --- | --- |
| GOV-101 | [KSDMA](https://sdma.kerala.gov.in/) | State alerts, hazard maps, official instructions, plans | Approved API/file exchange or public artifact with reuse permission |
| GOV-102 | [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/) | Pre-identified hazard/red-zone context | Versioned governed import; authority must say whether layer is operational |
| GOV-103 | [Wayanad district administration](https://wayanad.gov.in/) / DDMA | Safe zones, facility IDs, capacities, opening state, routes, landmarks, local contacts | Signed/approved operational package or authenticated API; this is mandatory for pilot |
| GOV-104 | Kerala Fire and Rescue Services / Police / Health official directories | Local government emergency numbers and facility contacts | Authority-maintained directory with effective dates |
| GOV-105 | Local-government and Public Works/Roads authority | Approved evacuation routes, closures, bridge/road constraints | Incident package or authenticated government feed |
| GOV-106 | Official Census/administrative boundary services | Gazetteer and jurisdiction boundaries | Approved government dataset; do not use it to infer live population |

State and district sources are jurisdiction-specific. Other pilots must replace GOV-101–106 with the corresponding authorized SDMA/DDMA and local bodies.

## 4. Language, accessibility, and technical dependencies

| ID | Source/dependency | Role | Boundary |
| --- | --- | --- | --- |
| DEP-001 | [AI4Bharat IndicConformer-600M-multilingual](https://huggingface.co/ai4bharat/indic-conformer-600m-multilingual) | Self-hosted ASR; model card states 600M parameters and 22 scheduled Indian languages | Benchmark on emergency speech, noise, accents, code-switching, and target hardware; model output is not authority |
| DEP-002 | AI4Bharat Indic Parler-TTS | Proposed self-hosted TTS | Exact repository, license, parameter size, supported-language list, safety, and deployment method remain an activation gate |
| DEP-003 | [BHASHINI](https://bhashini.gov.in/) | Government language-technology platform and possible approved fallback/integration | Confirm API, terms, retention, and service availability; do not silently send voice externally |
| DEP-004 | [Indian Sign Language Research and Training Centre](https://islrtc.nic.in/) | Terminology/dictionary and review partner for ISL assets | A dictionary is not automatically an emergency instruction pack; human Deaf/ISL review required |
| DEP-005 | [GIGW 3.0](https://guidelines.india.gov.in/gigw3/) | Government-web quality/accessibility requirements | Test applicable conformance |
| DEP-006 | [WCAG 2.2](https://www.w3.org/TR/WCAG22/) | AA accessibility target | External standard; manual tests required |
| DEP-007 | [OASIS Common Alerting Protocol 1.2](https://docs.oasis-open.org/emergency/cap/v1.2/CAP-v1.2.html) | Alert parsing semantics | SACHET implementation details and government policy control |

AI4Bharat is an IIT Madras research initiative, not a government operational alert authority. Its models may be used locally without making their outputs government data.

## 5. Connector activation gate

For every source, record:

1. Government owner and official domain/contact.
2. Legal/reuse basis and production access approval.
3. Endpoint/file mechanism, authentication, allow-listing, quota, and cache rules.
4. Geographic, hazard, temporal, and language coverage.
5. Field schema, units, CRS, precision, null/NoData, severity vocabulary, and update/cancel semantics.
6. Freshness target, observed update cadence, outage behavior, and escalation contact.
7. One permitted pilot-area sample with checksum and capture time.
8. Validation, conflict precedence, source attribution, retention, and redaction rules.
9. Contract and failure tests.
10. Named government operational owner approving activation.

States: `DISCOVERED -> ACCESS_REQUESTED -> SAMPLE_ACQUIRED -> VALIDATED -> AUTHORIZED -> OPERATIONAL -> SUSPENDED/RETIRED`. Only `OPERATIONAL` sources may drive citizen guidance.

## 6. Data precedence

1. Current incident-specific DDMA/SDMA direction.
2. Current SACHET CAP alert/update/cancel from the competent authority.
3. Current specialized warning from the competent central agency.
4. Approved operational zone/route/facility package.
5. Supporting official observation or reference layer.

Precedence does not permit silent merging. Conflicts are shown to operators, the safer official instruction is not guessed, and citizen guidance enters a clear degraded state until resolved.

## 7. Explicit exclusions

No Sentinel/Copernicus, Google Maps, Mapbox, HERE, private weather API, social media, crowdsourcing, private shelter directory, telecom-derived location, advertising analytics, or data-broker input may supply production disaster truth. A non-government open-source library may render or process official data locally if it sends no operational/user data to a private service.
