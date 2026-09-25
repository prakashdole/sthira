# Privacy Controls, Data Minimization, and Retention Architecture (M04)

**Phase / Task**: M04 — Minimal operator workflow, privacy controls, and notification integration  
**Status**: SPECIFIED_AND_IMPLEMENTED  
**Governance**: Rules 8, 14, 15 (GEMINI.md / rules.md); Open Decision O10 (open-decisions.md); Digital Personal Data Protection Act (DPDPA 2023) alignment

---

## 1. Executive Summary & Product Boundary

Sthira v2 operates under strict data minimization and purpose limitation principles. Citizen location telemetry and voice audio are treated as ephemeral operational signals, not persistent data assets. 

Sthira v2 explicitly rejects continuous citizen surveillance, automatic background geofencing, and remote tracking dashboards. All capacity mutations and arrival records require deliberate, un-coerced citizen actions.

---

## 2. Core Privacy Invariants (O10 Alignment)

### 2.1 Zero Continuous Background Location Tracking
- **Foreground Only**: Location tracking operates strictly in the active foreground tab/screen.
- **Explicit Citizen Opt-In**: The client requests location permission only after explicit citizen gesture (`Start Journey Assistance`).
- **Immediate Revocation**: Stopping tracking, closing the tab, or confirming arrival immediately tears down the `navigator.geolocation` watch.
- **No Background Surveillance**: Background service workers, persistent background daemons, or silent location beaconing are prohibited. If an app or tab is backgrounded, it displays "Last updated [time]" upon return and honestly notes that tracking was paused.

### 2.2 Ephemeral In-Memory Location Signals
- **Zero Server Telemetry**: Client GPS coordinates are **never** transmitted in periodic telemetry streams or logged to disk.
- **Local Uncertainty Evaluation**: Proximity to destination is evaluated deterministically on the client device using haversine mathematics (`computeDistanceMeters`).
- **Uncertainty Safeguards**: Positions with horizontal accuracy $> 100\text{ m}$ or timestamp age $> 30\text{ s}$ are classified as `INACCURATE` or `STALE` and cannot trigger a near-destination advisory.

### 2.3 Explicit Arrival Confirmation (No Automated Geofencing)
- Proximity detection ($< 150\text{ m}$) triggers an **advisory prompt only** ("You are near the shelter").
- Arrival is confirmed **solely via explicit physical touch / keyboard action** (`Confirm Arrival` button).
- Geofence triggers cannot automatically mutate shelter capacity, infer citizen welfare, or mark check-in.

### 2.4 Zero Raw Audio & Transcript Retention
- **Raw Audio Retention = 0 ms**: Audio recorded via microphone is buffered in memory only during recording. Once transmitted or processed, buffers are immediately garbage-collected.
- **No Speech Storage**: Voice Map Control utilizes self-hosted IndicConformer and Sarvam models without recording caller audio to disk or training pools.
- **Privacy Notice**: Citizens are visibly reminded: *"Voice audio is processed ephemerally for navigation commands only and is never stored."*

---

## 3. Remote Tracking Dashboard Non-Implementation Disclosure

In accordance with PRD and TRD boundary specifications:
- **No Remote Live Tracking Dashboard**: Sthira v2 does **not** implement a remote citizen live-tracking dashboard for operators or third parties.
- **Rationale**: Continuous location aggregation creates severe civil liberties risks during disasters, vulnerable to misuse, interception, and coercion.
- **Supported Operator Surface**: The operator surface is restricted to:
  1. Authoritative source status inspection (`OPERATIONAL`, `SUSPENDED`, `RETIRED`).
  2. Consequential source quarantine (`QUARANTINED`).
  3. Audited administrative stay corrections (e.g. party size reduction when family members are transferred to medical facilities).

---

## 4. Legal Audit Log Retention vs Citizen Telemetry Privacy

A fundamental architectural distinction exists between **regulatory disaster audit logging** and **citizen telemetry privacy**:

| Dimension | Administrative Actions (Audit Log) | Citizen Telemetry (GPS / Audio) |
| --- | --- | --- |
| **Data Scope** | Source publish/revoke/quarantine, stay party corrections, operator session issuance | Real-time coordinates, bearing, speed, voice recordings, raw transcripts |
| **Legal Basis** | Disaster Management Act compliance, evidentiary integrity, chain of custody | Consent-based foreground guidance under DPDPA 2023 |
| **Storage Architecture** | Append-only, tamper-evident SHA-256 hash chain (`audit_events` table) | In-memory ephemeral buffers only; zero database tables |
| **Identifiers** | Verified operator subject, grant ID, session ID, timestamp, cryptographic hash | Anonymous session token (`SES-...`); zero citizen PII |
| **Retention Period** | 7 years (statutory administrative retention) | **Zero ms** post-session termination |
| **Deletion Policy** | Immutable (legal requirement; cannot claim deletion on audit trail) | Auto-purged upon session end / tab close |

---

## 5. Privacy Policy & Disclosure Matrix for Drills and Mobile Clients

1. **Pre-Drill Disclosure**: Facilitators must brief all drill participants:
   > *"Sthira does not track your location in the background or record your voice. All navigation assistance is foreground-only and stops when you leave or confirm arrival."*
2. **Account / Session Isolation**:
   - Citizen sessions are identified by transient, client-generated IDs (`SES-...`) stored in `sessionStorage`.
   - Logging out or clearing browser storage completely discards all local keys.
3. **No Commercial or Third-Party Analytics**:
   - Zero Google Analytics, Firebase Analytics, Meta Pixel, or commercial SDKs.
   - Zero external map tile harvesting (MapLibre consumes verified local/container vectors only).
