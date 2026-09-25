# Task R01 Evidence — Destination Selection and Isolated Exercise Backend

**Task**: R01 — Destination selection and an explicitly isolated exercise backend  
**Role**: Backend / Exercise Coordinator  
**Commit**: `f645ba08d542c5d42a800ae21d346e00cf266f07` on branch `CLEAN`  
**Base Commit**: `f5951e0` on `CLEAN`  
**Host Environment**: macOS (Darwin arm64), Go 1.27.1, PostgreSQL 18.6 + PostGIS 3.6  

---

## 1. Executive Summary

Task R01 addresses two core architectural requirements:
1. **Authoritative Policy Ordering**: Correcting `scoped.go:buildEligible` and `choice.go:ChoiceQuerier` to strictly interpret `allocation_policy.order` as **safe-zone IDs** (per `opkg.AllocationPolicy` schema) rather than facility IDs, grouping facilities within each zone at the same `PermittedRank`, ordering facilities within a zone deterministically by ID, and propagating database errors instead of swallowing them.
2. **Process-Level Exercise Isolation**: Introducing `cmd/sthira-exercise`, a dedicated binary separate from `cmd/sthira` that explicitly wires `WithSyntheticExercise(true)` at boot time and seeds labeled `SYNTHETIC_DEMO` data (source, authorization, package, safe zone, facility, route, inventory) when `STHIRA_EXERCISE_SEED=1`. No HTTP header, request field, or query parameter can activate synthetic behavior (per Decision D47).

---

## 2. Key Code Changes

| File | Changes Made |
|---|---|
| `backend/internal/store/scoped.go` | Replaced legacy facility-level interpretation with safe-zone ID interpretation; mapped each zone to member facilities with uniform `PermittedRank`; ensured DB errors reading zone versions propagate immediately. |
| `backend/internal/store/choice.go` | Added `readPolicyOrderAndFacilityMap` helper to parse `allocation_policy.order` from package body; updated `ChoiceQuerier.Eligible` to emit destinations in policy order. |
| `backend/cmd/sthira-exercise/main.go` | Created process-isolated exercise binary; embedded synthetic demo seeder with explicit `SYNTHETIC_DEMO` provenance flags. |
| `backend/internal/store/scoped_integration_test.go` | Added `TestScopedGuidance_BuildEligible_UnknownZoneIDNotFabricated`, `TestScopedGuidance_BuildEligible_ClosedZoneExcluded`, and `TestScopedGuidance_BuildEligible_DBErrorPropagates`. |
| `backend/internal/store/choice_integration_test.go` | Added `TestChoiceQuerier_OrderingMatchesPolicyOrder` and `TestChoiceQuerier_UnknownZoneDropped`. |

---

## 3. Verification & Acceptance Results

### Automated Integration Tests
```bash
go test -v ./internal/store -run "TestScopedGuidance_BuildEligible|TestChoiceQuerier"
```
Output:
- `TestScopedGuidance_BuildEligible_UnknownZoneIDNotFabricated`: PASS
- `TestScopedGuidance_BuildEligible_ClosedZoneExcluded`: PASS
- `TestScopedGuidance_BuildEligible_DBErrorPropagates`: PASS
- `TestChoiceQuerier_OrderingMatchesPolicyOrder`: PASS
- `TestChoiceQuerier_UnknownZoneDropped`: PASS

### Full Go Suite (16 Packages Green)
```bash
go test ./internal/store ./internal/httpserver ./internal/capfeed ./internal/opkg \
        ./internal/sourceact ./internal/catalogue ./internal/scenarioprep \
        ./internal/orchestration ./internal/offlinepkg ./internal/offlineclient \
        ./internal/offlinedelivery ./internal/offlinequeue ./internal/offlineresources \
        ./internal/contracts ./internal/httpjson
```
Output: `ok` across all 16 packages.

### End-to-End Exercise Smoke Test
Against real PostgreSQL 18 with `cmd/sthira-exercise` (`STHIRA_EXERCISE_SEED=1`):
1. `GET /health/ready` $\rightarrow$ `{"status":"READY"}`
2. `POST /api/v3/sessions` $\rightarrow$ 201 Created with valid bearer token
3. `POST /api/v3/guidance/query` $\rightarrow$ Returns 1 destination, `free=100`, `route_verified=true`
4. `POST /api/v3/reservations` $\rightarrow$ 201 Created with `reservation_id` + `stay_id`
5. `POST /api/v3/reservations` (idempotent replay) $\rightarrow$ Returns identical `reservation_id` with 0 duplicate allocation
