---
title: Sthira Citizen Emergency Guidance — Parameter Registry
document_id: STHIRA-PARAMETERS
version: 2.0
status: Baseline schema; operational values require approval
as_of: 2026-09-11
---

# Parameter Registry

## 1. Governance

No operational threshold is hard-coded without an owner, jurisdiction, effective date, source, approval, and review trigger. Missing required values fail closed. Values below are schemas or engineering targets, not government policy unless marked approved.

| Parameter | Meaning | Owner | Baseline |
| --- | --- | --- | --- |
| `pilot_jurisdiction` | Active district/state scope | Product + DDMA | Wayanad, Kerala (proposed pilot) |
| `supported_ui_languages` | Human-reviewed interface languages | Government content owner | English + Malayalam for pilot; final approval open |
| `asr_language_set` | Enabled IndicConformer language codes | Voice owner | Enable only benchmarked pilot languages |
| `asr_acceptance_threshold.<lang>` | Minimum command acceptance confidence | Voice/accessibility owner | TBD by noisy-field evaluation |
| `tts_language_set` | Verified Indic Parler-TTS languages | Voice owner | TBD after model-card/license/runtime verification |
| `source_max_age.<source/product>` | Maximum operational age | Source authority | TBD per feed |
| `cap_poll_interval_seconds` | SACHET poll interval | NDMA agreement/operations | TBD; obey ETag and rate guidance |
| `assignment_ttl_seconds` | Reservation expiry | DDMA shelter policy | TBD |
| `capacity_mode` | `INFORMATIONAL` or `RESERVATION` | DDMA | Must be explicit |
| `location_precision_meters` | Stored/shared coordinate precision | Privacy + operations | Minimum needed; TBD |
| `raw_audio_retention_seconds` | Voice retention after processing | Privacy owner | 0 by default |
| `cache_package_ttl` | Offline guidance validity | Source authority | Must not exceed source expiry |
| `emergency_number` | Dialler target | Government directory owner | 112 nationally; local overrides/additions approved |
| `map_camera_duration_ms` | Nonessential camera animation | UX | 0 when reduced motion; otherwise tested |
| `arrival_confirmation_modes` | Permitted confirmation methods | Product/DDMA | Touch/keyboard baseline; geofence disabled |

## 2. Source configuration

Each connector configuration includes:

```text
source_id, authority, base_url, allowlisted_hosts, auth_method,
jurisdiction, hazard_types, polling_policy, cache_headers,
schema_version, severity_mapping, crs, max_age, timeout,
retry_policy, circuit_breaker, attribution, contact, status
```

Secrets are referenced from a secret manager, never committed.

## 3. Facility and route configuration

Government operational packages supply:

```text
facility_id, facility_version, name, geometry, total_capacity,
status, accessibility, contact, effective_from, expires_at,
route_ids, allocation_priority, authority, signature/checksum
```

Sthira does not provide defaults for total capacity, safety, opening status, route, or allocation priority.

## 4. Performance targets to validate

- Cached first useful screen: p95 ≤ 3 s on the pilot test profile.
- Capacity transaction: p95 ≤ 1 s under pilot peak load.
- Voice map command after speech end: p95 target ≤ 2.5 s on target inference hardware.
- Availability: 99.9% monthly during activated pilot windows.

These remain engineering targets until load, device, network, and model benchmarks establish feasibility.
