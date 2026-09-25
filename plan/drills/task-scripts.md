# Participant Drill Task Scripts (Q01)

**Phase / Task**: Q01 — Regional scenarios, languages and participant-ready drill inputs  
**Format**: Structured Facilitator Prompts, Participant Instructions, and Evaluation Metrics  
**Execution Context**: Supervised controlled indoor usability drill on mobile / laptop interfaces

---

### Task 1: Immediate Emergency Evacuation & Stay Selection
- **Scenario**: Severe flash flood warning in high-risk river basin (e.g., Meppadi, Wayanad / Beas Basin, Mandi).
- **Participant Prompt**: *"You have received an urgent alert on your phone. Find the nearest available safe shelter for immediate stay and review the route to reach it."*
- **Required Steps**:
  1. Open citizen guidance interface and inspect active alert banner.
  2. Speak voice query in preferred language (*"എവിടെയാണ് സുരക്ഷിതമായ അഭയകേന്ദ്രം?"* / *"निकटतम सुरक्षित राहत शिविर कहाँ है?"*) or tap Destination list.
  3. Select the top-ranked safe facility with verified available capacity.
  4. Confirm immediate stay hold and inspect turn-by-turn government-approved route.
- **Pass Criteria**:
  - Choice completed within 45 seconds.
  - Selected facility is within an approved safe zone (not in active red hazard zone).
  - Explicit reservation holds 1 capacity unit without double allocation.

---

### Task 2: 7–30 Day Temporary Stay Selection
- **Scenario**: Ongoing slope instability warning requiring 7–30 day temporary relocation.
- **Participant Prompt**: *"Your home is in an advisory landslide zone requiring temporary relocation for up to two weeks. Select an approved temporary facility that supports a 14-day stay."*
- **Required Steps**:
  1. Select temporary stay filter (7–30 days).
  2. Inspect available facility cards, noting facility services (medical, food, accessibility).
  3. Confirm 14-day reservation hold.
- **Pass Criteria**:
  - Participant distinguishes immediate emergency shelter from extended temporary stay.
  - Server successfully commits reservation with valid start date and requested duration.

---

### Task 3: Ambiguous Village Name Resolution
- **Scenario**: Citizen searches for a village name that exists in multiple taluks / districts (e.g., "Meppadi" in Wayanad vs "Meppadi" in adjacent district).
- **Participant Prompt**: *"Search for 'Meppadi' to find evacuation instructions for your location."*
- **Required Steps**:
  1. Enter or speak "Meppadi".
  2. Interface detects ambiguous place candidates and displays distinct selection chips with administrative district / taluk labels.
  3. Participant selects the correct district match.
- **Pass Criteria**:
  - System does NOT arbitrarily guess a single candidate or route to the wrong district.
  - Candidate chips clearly display disambiguating administrative hierarchy.
  - Correct jurisdiction is bound to subsequent guidance queries.

---

### Task 4: Route Revocation & Facility Transfer During Transit
- **Scenario**: While citizen is travelling, an official route closure is declared due to bridge damage or rising floodwaters.
- **Participant Prompt**: *"While following the route on your map, observe any updates that appear on your screen and take appropriate action."*
- **Required Steps**:
  1. System receives official route revocation / hazard polygon update.
  2. Red route line is revoked with prominent "Route Closed by Authority" warning banner.
  3. An alternate authorized route or transfer facility is presented.
  4. Citizen accepts transfer to secondary facility.
- **Pass Criteria**:
  - Navigation immediately halts on revoked route (no dangerous routing fallback).
  - Previous facility hold is cleanly transferred to the new facility without capacity leakage.

---

### Task 5: Offline Cold-Start & Network Interruption
- **Scenario**: Network connectivity drops completely while citizen is travelling outside mobile coverage.
- **Participant Prompt**: *"Your device loses mobile internet connection. Verify that you can still view your route and shelter directions."*
- **Required Steps**:
  1. Facilitator switches test device to Airplane mode.
  2. Citizen opens or refreshes application.
  3. App loads cached regional package (`SYNTHETIC_DEMO`), displaying cached route, facility details, and non-map textual directions.
  4. Notice indicates offline status with timestamp of cached data.
- **Pass Criteria**:
  - Complete non-map guidance remains readable offline.
  - App displays honest offline indicators without claiming live authority updates.

---

### Task 6: Caregiver / Family Group Reservation
- **Scenario**: Caregiver evacuating with 3 family members (party size = 4).
- **Participant Prompt**: *"You are evacuating with 3 other family members. Reserve space for 4 people at a safe shelter."*
- **Required Steps**:
  1. Adjust party size selector from 1 to 4.
  2. Review facility capacity indicators (verifying facility has >= 4 beds remaining).
  3. Submit reservation.
- **Pass Criteria**:
  - Request payload explicitly includes `party_size: 4`.
  - Database atomically decrements exactly 4 units of remaining capacity.
  - If fewer than 4 beds remain, system returns explicit `CAPACITY_CONFLICT` rather than partial overbooking.

---

### Task 7: Proximity Advisory & Explicit Arrival Confirmation
- **Scenario**: Citizen reaches the safe facility perimeter.
- **Participant Prompt**: *"You have reached the gate of the shelter. Confirm your arrival."*
- **Required Steps**:
  1. Foreground tracking detects location within proximity threshold (<=150m) of destination geometry.
  2. Screen displays advisory prompt: *"You appear to be near the facility. Tap Confirm Arrival once inside."*
  3. Citizen presses the explicit "Confirm Arrival" button.
  4. Interface transitions to `ARRIVAL_CONFIRMED`.
- **Pass Criteria**:
  - Arrival is NOT automatically triggered by background geofencing.
  - Idempotent event `POST /api/v3/reservations/{id}/events` transitions reservation from HOLD to OCCUPIED.
  - Repeated button presses produce a single atomic state transition without duplicate decrement.
