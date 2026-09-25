import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  validateStayCorrection,
  validateSourceTransition,
  validateSourceQuarantine,
  validateNotificationPayload,
  revalidateNotification,
  requestOperatorSession,
  type NotificationPayload,
} from './operator.ts';

test('validateStayCorrection: rejects party size < 1', () => {
  const result = validateStayCorrection(4, 0, 'Transferred dependent');
  assert.equal(result.valid, false);
  assert.match(result.error || '', /positive integer/);
});

test('validateStayCorrection: rejects non-integer party size', () => {
  const result = validateStayCorrection(4, 2.5, 'Transferred dependent');
  assert.equal(result.valid, false);
  assert.match(result.error || '', /positive integer/);
});

test('validateStayCorrection: rejects party size greater than current (cannot expand capacity)', () => {
  const result = validateStayCorrection(3, 5, 'Adding extra relatives');
  assert.equal(result.valid, false);
  assert.match(result.error || '', /cannot expand party size/);
});

test('validateStayCorrection: rejects missing or too-short reason', () => {
  const result1 = validateStayCorrection(4, 2, '');
  assert.equal(result1.valid, false);
  assert.match(result1.error || '', /at least 5 characters/);

  const result2 = validateStayCorrection(4, 2, 'fix');
  assert.equal(result2.valid, false);
  assert.match(result2.error || '', /at least 5 characters/);
});

test('validateStayCorrection: accepts valid reduction with detailed reason', () => {
  const result = validateStayCorrection(4, 2, 'Two members transferred to community hospital');
  assert.equal(result.valid, true);
  assert.equal(result.error, undefined);
});

test('validateSourceTransition: enforces non-empty source ID and allowed target states', () => {
  const res1 = validateSourceTransition('', 'OPERATIONAL', 'SUSPENDED');
  assert.equal(res1.valid, false);
  assert.match(res1.error || '', /Source ID is required/);

  const res2 = validateSourceTransition('gov:cwc:kl:001', 'OPERATIONAL', 'INVALID_TARGET' as any);
  assert.equal(res2.valid, false);
  assert.match(res2.error || '', /OPERATIONAL, SUSPENDED, or RETIRED/);
});

test('validateSourceTransition: rejects transition to identical state', () => {
  const res = validateSourceTransition('gov:cwc:kl:001', 'OPERATIONAL', 'OPERATIONAL');
  assert.equal(res.valid, false);
  assert.match(res.error || '', /already in OPERATIONAL state/);
});

test('validateSourceTransition: accepts valid transition', () => {
  const res = validateSourceTransition('gov:cwc:kl:001', 'OPERATIONAL', 'SUSPENDED');
  assert.equal(res.valid, true);
  assert.equal(res.error, undefined);
});

test('validateSourceQuarantine: enforces detailed consequential justification', () => {
  const res1 = validateSourceQuarantine('gov:cwc:kl:001', '');
  assert.equal(res1.valid, false);
  assert.match(res1.error || '', /at least 10 characters/);

  const res2 = validateSourceQuarantine('gov:cwc:kl:001', 'broken');
  assert.equal(res2.valid, false);
  assert.match(res2.error || '', /at least 10 characters/);

  const res3 = validateSourceQuarantine('gov:cwc:kl:001', 'Gauge telemetry telemetry failure reported by district collector');
  assert.equal(res3.valid, true);
});

test('validateNotificationPayload: accepts minimal identifier payload', () => {
  const validPayload: NotificationPayload = {
    version: '1.0',
    notification_id: 'NOTIF-2026-001',
    category: 'INCIDENT_UPDATE',
    incident_id: 'INC-KL-001',
    package_id: 'PKG-KL-001',
    jurisdiction: 'KL-WYD',
    timestamp: new Date().toISOString(),
  };
  const res = validateNotificationPayload(validPayload);
  assert.equal(res.valid, true);
});

test('validateNotificationPayload: rejects sensitive GPS coordinates and route geometries (O12)', () => {
  const payloadWithCoords = {
    version: '1.0',
    notification_id: 'NOTIF-2026-001',
    category: 'EVACUATION_NOTICE',
    incident_id: 'INC-KL-001',
    package_id: 'PKG-KL-001',
    jurisdiction: 'KL-WYD',
    timestamp: new Date().toISOString(),
    latitude: 11.55, // FORBIDDEN
    longitude: 76.12, // FORBIDDEN
  };
  const res = validateNotificationPayload(payloadWithCoords);
  assert.equal(res.valid, false);
  assert.match(res.error || '', /Sensitive data 'latitude' prohibited/);

  const payloadWithRoute = {
    version: '1.0',
    notification_id: 'NOTIF-2026-001',
    category: 'EVACUATION_NOTICE',
    incident_id: 'INC-KL-001',
    package_id: 'PKG-KL-001',
    jurisdiction: 'KL-WYD',
    timestamp: new Date().toISOString(),
    route_polyline: 'enc:xyz', // FORBIDDEN
  };
  const resRoute = validateNotificationPayload(payloadWithRoute);
  assert.equal(resRoute.valid, false);
  assert.match(resRoute.error || '', /Sensitive data 'route_polyline' prohibited/);
});

test('revalidateNotification: accepts fresh incident and rejects stale or revoked incident', () => {
  const payload: NotificationPayload = {
    version: '1.0',
    notification_id: 'NOTIF-01',
    category: 'INCIDENT_UPDATE',
    incident_id: 'INC-KL-001',
    package_id: 'PKG-KL-001',
    jurisdiction: 'KL-WYD',
    timestamp: new Date().toISOString(),
  };

  // Case 1: Fresh incident
  const fresh = revalidateNotification(payload, {
    incident_id: 'INC-KL-001',
    freshness: 'FRESH',
    is_revoked: false,
    expires_at: new Date(Date.now() + 3600000).toISOString(),
  });
  assert.equal(fresh.actionable, true);

  // Case 2: Revoked incident
  const revoked = revalidateNotification(payload, {
    incident_id: 'INC-KL-001',
    freshness: 'FRESH',
    is_revoked: true,
    expires_at: new Date(Date.now() + 3600000).toISOString(),
  });
  assert.equal(revoked.actionable, false);
  assert.match(revoked.reason, /revoked/);

  // Case 3: Stale incident
  const stale = revalidateNotification(payload, {
    incident_id: 'INC-KL-001',
    freshness: 'STALE',
    is_revoked: false,
    expires_at: new Date(Date.now() + 3600000).toISOString(),
  });
  assert.equal(stale.actionable, false);
  assert.match(stale.reason, /stale/);

  // Case 4: Expired alert
  const expired = revalidateNotification(payload, {
    incident_id: 'INC-KL-001',
    freshness: 'FRESH',
    is_revoked: false,
    expires_at: new Date(Date.now() - 60000).toISOString(),
  });
  assert.equal(expired.actionable, false);
  assert.match(expired.reason, /expired/);

  // Case 5: Mismatched incident ID
  const mismatched = revalidateNotification(payload, {
    incident_id: 'INC-OTHER-999',
    freshness: 'FRESH',
    is_revoked: false,
    expires_at: new Date(Date.now() + 3600000).toISOString(),
  });
  assert.equal(mismatched.actionable, false);
  assert.match(mismatched.reason, /does not match/);
});

test('requestOperatorSession: handles 503 fail-closed when no IdP configured', async () => {
  // Mock fetch simulating production server without IdP verifier
  const originalFetch = globalThis.fetch;
  try {
    globalThis.fetch = async () =>
      new Response(
        JSON.stringify({
          error: {
            code: 'DATA_UNAVAILABLE',
            message: 'operator issuance unavailable: no trusted identity verifier configured',
          },
        }),
        { status: 503, headers: { 'Content-Type': 'application/json' } }
      );

    const res = await requestOperatorSession('http://localhost:8080');
    assert.equal(res.ok, false);
    assert.equal(res.status, 503);
    assert.match(res.error || '', /no trusted identity verifier configured/);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
