# Sthira v2 Standing Instructions and Memory

## Product boundary

Sthira is a citizen-facing interface over existing government disaster systems. It consumes authoritative alerts and pre-approved red zones, safe zones, routes, facility capacities, allocation policies, and instructions. It improves comprehension and action; it does not predict or authorize.

The active baseline is v2. Prior permanent-relocation, land, candidate-site, scheme, and beneficiary workflows are legacy code, not current requirements.

## Mandatory reading order

1. `prd.md`
2. `rules.md`
3. `trd.md`
4. `architecture.md`
5. `source-register.md`
6. `decisions.md`
7. `open-decisions.md`
8. `phases.md` and `plan.md`

## Standing engineering rules

- Use only government operational data in production. Open-source code and self-hosted models are processing dependencies, not sources of truth.
- NDMA SACHET CAP is the primary alert backbone unless an accepted ADR changes it.
- Preserve official source, identifier, version, issue/effective/expiry times, and raw artifact.
- Fail closed on missing, stale, invalid, conflicting, unauthorized, or out-of-coverage data.
- Never predict red zones, safe zones, evacuation routes, facility capacity, or severity.
- Use only government-approved routes and allocation policies.
- Implement capacity as atomic idempotent events; never allow negative remaining capacity.
- Arrival requires explicit touch/keyboard confirmation in baseline. Do not implement geofencing.
- Voice Map Control uses self-hosted IndicConformer ASR, deterministic allow-listed intents, and visible/touch alternatives.
- Treat IndicConformer and Indic Parler-TTS honestly as AI/ML. They never make emergency decisions.
- Voice cannot call, confirm arrival, change capacity, or override an assignment.
- Open 112/local official number through the device dialler only after explicit citizen action.
- Use ISL terminology and approved signed media; never label English text as sign language.
- Keep the map, voice, and audio optional: text/non-map guidance must remain complete.
- Do not add private maps, weather, routing, speech, analytics, advertising, or location services to the production path.
- Minimize and purpose-limit location and voice. Raw audio retention defaults to zero.
- Maintain demo/shadow/pilot/production isolation and unmistakable synthetic labels.
- Use simple architecture and existing language/framework capabilities before adding services/dependencies.
- Add tests before or with behavior, especially for CAP updates/cancels, stale data, capacity races, retries, prohibited voice commands, offline flow, and accessibility.
- Record material technical/product decisions in `decisions.md`; never bury policy assumptions in code.
- Do not close `open-decisions.md` without named-owner evidence.
- Do not modify any `.txt` file unless explicitly asked.

## Definition of honest completion

Documentation completion does not mean the application is migrated. A live claim requires authorized source access, operational government data, model deployment and benchmarks, language/ISL approval, privacy/security review, runbooks, drills, and government sign-off.
