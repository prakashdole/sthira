---
title: Sthira v2 Working Memory
version: 2.0
as_of: 2026-09-11
---

# Working Memory

## Durable product truth

- Sthira v2 is a citizen emergency-guidance bridge over existing government systems.
- It consumes only government operational data and does not source disaster truth from private companies.
- Government authorities own hazard/red zones, safe zones, routes, capacities, allocation policy, instructions, and response.
- Sthira does not predict any of those items.
- NDMA SACHET CAP is the primary alert backbone; specialized government sources add labeled context.
- The “God's view” concept is named Voice Map Control: citizens can move/focus the map with voice.
- ASR is AI4Bharat IndicConformer-600M-multilingual, self-hosted and gated by per-language testing.
- TTS is proposed AI4Bharat Indic Parler-TTS, self-hosted and gated by exact model/license/runtime verification.
- Voice commands are deterministic after transcription and cannot perform consequential actions without touch confirmation.
- Emergency calling opens the device dialler to 112/approved local number; Sthira does not claim dispatch.
- Arrival is an explicit Yes/No prompt. Yes changes capacity exactly once for the confirmed party size.
- Automatic geotag/geofence arrival is future-only.
- Signed-language baseline in India is ISL, not ASL, unless actual ASL media is supplied.
- Wayanad, Kerala remains the proposed pilot, pending government agreement.
- Existing application code implements the superseded permanent-relocation prototype and must not be mistaken for v2 completion.

## Working discipline

- Read `prd.md`, `rules.md`, `trd.md`, `architecture.md`, and `decisions.md` before coding.
- Record material decisions in `decisions.md`; track unresolved policy in `open-decisions.md`.
- Use synthetic fixtures until a source reaches `OPERATIONAL` in `source-register.md`.
- Preserve user files and all `.txt` files.
- Verify emergency behavior in the running app, not only unit tests.
