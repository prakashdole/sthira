# Notification Transport, Minimal Payload Contract, and Revalidation Protocol (M04)

**Phase / Task**: M04 — Minimal operator workflow, privacy controls, and notification integration  
**Status**: CONTRACT_FROZEN_AND_IMPLEMENTED (Server push credentials NOT_RUN per O12)  
**Governance**: Rules 4, 14 (GEMINI.md / rules.md); Open Decision O12 (open-decisions.md)

---

## 1. Context & Architectural Boundary

During severe disasters, cellular and push delivery networks experience extreme latency, packet loss, and cell tower degradation. Notification systems cannot guarantee timely delivery. 

Consequently:
1. **Core Experience Independence**: Core foreground guidance, offline package browsing, and stay reservations function fully without push notification permissions.
2. **No Guaranteed Push Claim**: Sthira never guarantees instantaneous push delivery.
3. **Client Revalidation Invariant**: Push notifications are treated as untrusted wakeups. A client receiving a notification MUST revalidate current incident freshness against the authoritative `/api/v3` backend before presenting actionable emergency instructions to the citizen.

---

## 2. Notification Transport Architecture (O12)

```
[ Authoritative Source Transition / Alert Update ]
                      │
                      ▼
        [ Sthira Go Backend API ]
                      │
      ┌───────────────┴───────────────┐
      ▼                               ▼
[ Apple APNs Provider ]      [ Google FCM Provider ]
 (Server-side certs only)     (Server-side keys only)
      │                               │
      ▼                               ▼
  [ iPhone ]                      [ Android ]
      │                               │
      └───────────────┬───────────────┘
                      ▼
       [ Wakeup / Minimal Payload ]
                      │
                      ▼
     [ Revalidation GET /api/v3/... ]
                      │
       ┌──────────────┴──────────────┐
       ▼                             ▼
 [ Fresh & Valid ]           [ Stale / Revoked ]
         │                           │
         ▼                           ▼
[ Actionable Citizen Alert ]  [ Suppressed / Warned ]
```

- **Credential Isolation**: APNs p8 auth keys and FCM service account tokens reside strictly on the backend server. No provider credentials or service account JSON files are ever packaged into citizen clients.
- **Provider Status**: In local development and drill environments, live push provider activation is recorded as `NOT_RUN` (pending enterprise developer account provisioning under O12).

---

## 3. Minimal Notification Payload Specification

To protect citizen privacy, avoid payload interception risks, and prevent out-of-date instructions from lingering on lockscreens, the notification payload is strictly minimal.

### 3.1 Allowed Schema (JSON)

```json
{
  "version": "1.0",
  "notification_id": "NOTIF-2026-KL-001",
  "timestamp": "2026-09-23T12:00:00Z",
  "category": "INCIDENT_UPDATE",
  "incident_id": "INC-2026-KL-001",
  "package_id": "PKG-EXERCISE-01",
  "jurisdiction": "KL-WYD",
  "deep_link": "sthira://incident/INC-2026-KL-001"
}
```

### 3.2 Category Enumeration
- `INCIDENT_UPDATE`: Authoritative alert level or evacuation zone geometry has changed.
- `STAY_UPDATE`: Administrative stay status change (e.g. facility closure or administrative party size correction).
- `EVACUATION_NOTICE`: New authoritative evacuation order issued for the jurisdiction.

### 3.3 Prohibited Fields (O10 / O12 Violation)
The following fields are strictly prohibited from push payloads:
- `latitude` / `longitude` / `coordinates` (citizen or shelter precise coordinates)
- `route_polyline` / `route_geometry` (evacuation paths)
- `guaranteed_capacity` / `free_spaces` (stale shelter numbers)
- `citizen_name` / `citizen_phone` (citizen PII)

---

## 4. Client Revalidation & Deduplication Algorithm

When an incoming notification is received or opened via deep link:

1. **Payload Structure Validation**:
   - Verify presence of `notification_id`, `incident_id`, `package_id`, and `jurisdiction`.
   - Assert zero prohibited sensitive fields (`validateNotificationPayload`).

2. **Deduplication Check**:
   - Check local persistent store for `notification_id`. If previously processed within last 24 hours, discard.

3. **Freshness & Revocation Revalidation (`revalidateNotification`)**:
   - Client sends authenticated query to `GET /api/v3/guidance` or `GET /api/v3/incidents/{incident_id}`.
   - **Mismatched Incident**: If server active incident differs from payload, discard notification.
   - **Revoked Incident**: If incident was revoked by disaster authorities, suppress notification and clear obsolete route.
   - **Stale Incident**: If server freshness is `STALE` or timestamp is expired, suppress actionable prompt and display "Stale Alert Notice".
   - **Fresh Incident**: Only when backend reports `FRESH` and unexpired does the client update the map workspace, alert banner, and audio narration.

---

## 5. Verification & Test Evidence

All client-side payload validation rules and revalidation invariants are implemented in `frontend/v2/src/operator.ts` and verified by `frontend/v2/src/operator.test.ts`:
- Rejection of coordinate/route-bearing payloads: PASS
- Handling of fresh vs stale vs revoked incidents: PASS
- Handling of expired alerts: PASS
- Handling of mismatched incident IDs: PASS
