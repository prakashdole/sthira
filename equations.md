---
title: Sthira Citizen Emergency Guidance — Controlled Calculations
document_id: STHIRA-EQUATIONS
version: 2.0
status: Normative
as_of: 2026-09-11
---

# Controlled Calculations

Sthira contains no hazard-prediction, red-zone-generation, site-suitability, or master-risk equation. It preserves official values and performs only interface/operational calculations below.

## EQ-001 Remaining capacity

```text
remaining = official_total
          + sum(authorized_adjustments)
          - sum(active_reservations)
          - sum(confirmed_occupancy_not_already_reserved)
```

The implementation may model confirmed arrival as conversion of a reservation rather than an additional subtraction. It must never count the same party twice. Inputs are integer persons; result must be non-negative.

## EQ-002 Assignment eligibility

```text
eligible(zone, party) =
  zone.status == OPEN
  AND zone.version is active
  AND approved_route exists and is active
  AND remaining_capacity >= party.size
  AND authority_policy permits assignment
```

Ordering between eligible zones is supplied by a versioned government policy. No Sthira distance/risk/quality score chooses a destination.

## EQ-003 Capacity transition

```text
apply(event) exactly once by event.idempotency_key
new_remaining = previous_remaining + event.delta
assert new_remaining >= 0
```

Arrival confirmation produces the policy-defined reservation-to-occupancy transition. Replays return the existing result.

## EQ-004 Freshness

```text
age_seconds = max(0, now_utc - source_observed_or_issued_at)
is_current = now_utc <= expires_at
             AND age_seconds <= configured_source_max_age
             AND validation_state == VALID
```

If the source supplies no expiry, only an authority-approved maximum age may be used. Missing policy means not current.

## EQ-005 Voice acceptance

```text
execute = intent in allow_list
          AND asr_confidence >= language_threshold
          AND required_slots_resolved_from_official_gazetteer
          AND command_is_non_consequential
```

Otherwise ask for clarification or show a confirmation screen. ASR confidence is not a safety probability.

## EQ-006 Display distance

Distance may be shown only if the route package contains official length, or as a clearly labeled geometric/display estimate. It cannot be used to claim safety or substitute for an approved route.

## Prohibited calculations

- Hazard likelihood or severity invented from precipitation.
- Red/safe-zone prediction.
- Facility safety or route safety score.
- Citizen vulnerability score.
- Opaque combined “God's view” score.
- Automatic arrival probability.
- Capacity inferred from building area, satellite imagery, or crowdsourcing.
