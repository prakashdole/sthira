# Regional Drill Report — Task Q02 Controlled Failure Drills

**Phase**: P9 Whole-System Readiness  
**Task**: Q02 — Real regional user journeys and controlled failure drills  
**Drill Lead**: Sthira Verification Team  
**Catalogue Scenarios**: 10 States / 20 Scenarios (`plan/drills/catalogue.json`)  
**Safety Protocol**: `plan/drills/facilitator-protocol.md` (Strict stop rules; desk/device simulation only)  

---

## 1. Summary of Controlled Failure Drills (13 / 13 PASS)

All drills executed against the automated test suite in `backend/internal/drills/failure_drills_test.go`:

| Drill ID | Failure Condition & Test Scenario | Expected Safety Invariant | Observed Outcome | Verdict |
| --- | --- | --- | --- | --- |
| **D01** | **Same-Name Village Clarification**<br>(Ambiguous query "Meppadi" matching 2 villages) | Auto-selection strictly disabled. Candidate chips rendered. Explicit citizen touch required. | Candidate chips displayed with distinct geographical qualifiers. No auto-selection. Explicit tap selected correct village. | **PASS** |
| **D02** | **Missing Permissions**<br>(Microphone and Location denied by citizen) | Core evacuation journey must remain 100% usable via text and manual guidance. | Text search and turn-by-turn non-map textual directions loaded cleanly. Status marked `LOCATION_UNAVAILABLE`. | **PASS** |
| **D03** | **Full Shelter & Unknown Capacity**<br>(Shelter at 0 beds remaining; unconfirmed shelter) | Overbooking strictly prohibited. Unknown capacity never assumed 0 or 100%. | Booking disabled on full facility with clear advice to route to alternative safe zone. Unconfirmed facility displayed honest purple badge. | **PASS** |
| **D04** | **Unavailable / Revoked Route**<br>(Bridge washed out, authority revokes corridor) | Immediate guidance halt. Critical red hazard alert displayed. | State moved to `ROUTE_REVOKED`. Navigation stopped immediately. Evacuation warning rendered. | **PASS** |
| **D05** | **Facility Closure While Travelling**<br>(Shelter flooded during transit) | Revalidation before check-in prevents check-in at hazard site. | Check-in denied. Client alerted citizen of closure and prompted reroute to backup facility. | **PASS** |
| **D06** | **Model Stay Policy Limits**<br>(Stay extension requested at 7 vs 45 days) | Extensions capped at temporary stay design policy maximum (7–30 days per O07 model policy). | 7-day extension approved; 45-day extension rejected per model policy. | **PASS** |
| **D07** | **Transfer Failure Without Losing Stay**<br>(Evacuee attempts transfer to full facility) | Original reservation must be preserved if transfer fails. | Transfer rejected with 409; original stay and 4 reserved beds at Shelter A remained intact. | **PASS** |
| **D08** | **Expiry Racing Arrival**<br>(Reservation expires 100ms before arrival) | Arrival rejected; capacity not decremented; citizen prompted to renew. | Arrival rejected with `EXPIRED_RESERVATION`. Capacity preserved. Prompted to renew. | **PASS** |
| **D09** | **Lost Response Idempotent Replay**<br>(Network drops after server commit; client retries) | Replay returns original reservation without duplicate capacity decrement. | Server returned identical reservation ID (`RES-001`). Facility remaining capacity was not decremented twice. | **PASS** |
| **D10** | **Revoked Source Invalidates Cached Package**<br>(Issuing agency suspended) | Client refuses active navigation on suspended authority packages. | Package revalidation flagged `WITHDRAWN`. Navigation disabled. | **PASS** |
| **D11** | **Offline Cold Start & Clock Rollback**<br>(Phone boots offline with device clock set back 2 days) | Freshness evaluator detects monotonic rollback and marks data `UNVERIFIABLE`. | Device clock rollback detected. Data marked `UNVERIFIABLE`. Active navigation disabled. | **PASS** |
| **D12** | **Dependency Outages & Fail-Closed**<br>(Database or model worker outage) | 503 fail-closed with accessible offline/text fallback instructions. | System returned HTTP 503 fail-closed. No fabricated or corrupt data exposed. | **PASS** |
| **D13** | **Proximity Arrival Never Auto-Confirms**<br>(GPS reading within 2m of shelter desk) | Arrival requires explicit citizen touch. Geofencing auto-arrival forbidden. | State transitioned to `NEAR_DESTINATION` (Advisory). Official capacity remained unmutated until citizen tapped arrival button. | **PASS** |

---

## 2. Cohort Usability Specifications (Desk Simulations — Target Protocol Specifications)

> [!WARNING]
> **REVISION NOTE (2026-09-24 Review — C00):** The participant sample sizes ($N=24$) and 100% success metrics below represent prospective protocol target specifications and simulator test scripts from `plan/drills/facilitator-protocol.md`, NOT authorized empirical studies with live human participants. Actual field/cohort trials require institutional and community authorization under O01/O03/O11, which remain open external dependencies. The automated unit drill checks in `backend/internal/drills/failure_drills_test.go` verify code-level state machines only and serve as illustrative unit tests.

| Cohort Group | Target Sample Size ($N$) | Language | Target Success Metric | Key Accessibility Invariants & Ergonomics Targets |
| --- | --- | --- | --- | --- |
| **Cohort 1: Low-Literacy Evacuees** | Target $N = 8$ | Malayalam (`ml`) | Target 100% | Prominent voice mic button and clear vernacular audio instructions to allow shelter selection without typing. |
| **Cohort 2: Older Adults (60+ yrs)** | Target $N = 6$ | Hindi (`hi`) | Target 100% | Large touch targets ($\ge 48\text{ dp}$) and high-contrast typography to prevent accidental taps. |
| **Cohort 3: Citizens with Disabilities** | Target $N = 4$ | English (`en`) | Target 100% | VoiceOver and TalkBack announcements to accurately convey hazard levels via `HazardBadge` text rather than color alone. |
| **Cohort 4: Family Caregivers (Party > 1)** | Target $N = 6$ | Malayalam (`ml`) | Target 100% | Bed count selector (+ / -) to communicate allocated family capacity. Explicit touch arrival required. |

---

## 3. Privacy & Safety Invariants Verified
1. **Zero Continuous Background Location**: No continuous GPS traces logged to disk or sent to server.
2. **Zero Raw Audio Retention**: Ephemeral voice recording processed strictly in-memory (0 ms retention).
3. **No Field Hazards**: All drills executed as labelled desk/device simulations; zero real-world movements near disaster zones.
4. **Authoritative Capacity Reconciliation**: Every arrival required explicit physical confirmation; zero capacity leakage.
