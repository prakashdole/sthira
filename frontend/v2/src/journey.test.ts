import test from 'node:test';
import assert from 'node:assert/strict';
import {
  computeDistanceMeters,
  evaluateProximity,
  transitionOnPosition,
  transitionOnArrival,
  transitionOnRevocation,
  evaluateArrivalConfirmation,
  shouldDropResponse,
  validateGuidanceSnapshot,
  mapGuidanceDestinations,
  resolveChoiceAgainstGuidance,
  buildReservationPayload,
  DEFAULT_JOURNEY_OPTIONS,
  type DestinationTarget,
  type PositionReading,
  type RawGuidanceDestination,
  type ResolvedDestinationChoice,
} from './journey.ts';

const DEMO_SHELTER: DestinationTarget = {
  id: 'FAC-DEMO-01',
  name: 'Demo Community Hall',
  longitude: 76.105,
  latitude: 11.570,
};

test('computeDistanceMeters calculates accurate distance between two points', () => {
  // Same point
  const zeroDist = computeDistanceMeters(76.105, 11.570, 76.105, 11.570);
  assert.equal(Math.round(zeroDist), 0);

  // User starting point in Ward 12 (~2.7 km away)
  const dist = computeDistanceMeters(76.123, 11.553, 76.105, 11.570);
  assert.ok(dist > 2600 && dist < 2800, `Expected distance ~2700m, got ${dist}`);

  // Invalid coordinates return NaN
  assert.ok(Number.isNaN(computeDistanceMeters(200, 11.5, 76.1, 11.5)));
  assert.ok(Number.isNaN(computeDistanceMeters(76.1, 95, 76.1, 11.5)));
});

test('evaluateProximity: detects citizen near destination with high accuracy and freshness', () => {
  const now = Date.now();
  // Position 40m away from destination
  const pos: PositionReading = {
    longitude: 76.1053,
    latitude: 11.5702,
    accuracyMeters: 10,
    timestamp: now,
  };

  const evalResult = evaluateProximity(pos, DEMO_SHELTER, DEFAULT_JOURNEY_OPTIONS, now);
  assert.equal(evalResult.isNear, true);
  assert.equal(evalResult.isStale, false);
  assert.equal(evalResult.isAccurateEnough, true);
  assert.equal(evalResult.reason, 'VERIFIED_NEAR');
  assert.ok(evalResult.distanceMeters < 100);
});

test('evaluateProximity: rejects position if accuracy is too low/uncertain (> 100m)', () => {
  const now = Date.now();
  // Close position, but terrible GPS accuracy (±250m)
  const pos: PositionReading = {
    longitude: 76.1053,
    latitude: 11.5702,
    accuracyMeters: 250,
    timestamp: now,
  };

  const evalResult = evaluateProximity(pos, DEMO_SHELTER, DEFAULT_JOURNEY_OPTIONS, now);
  assert.equal(evalResult.isNear, false, 'Low accuracy reading must not assert near destination');
  assert.equal(evalResult.isAccurateEnough, false);
  assert.equal(evalResult.reason, 'LOW_ACCURACY');
});

test('evaluateProximity: rejects stale position (> 30s old)', () => {
  const now = Date.now();
  // Close position, but timestamp is 45 seconds old
  const pos: PositionReading = {
    longitude: 76.1053,
    latitude: 11.5702,
    accuracyMeters: 15,
    timestamp: now - 45000,
  };

  const evalResult = evaluateProximity(pos, DEMO_SHELTER, DEFAULT_JOURNEY_OPTIONS, now);
  assert.equal(evalResult.isNear, false, 'Stale reading must not assert near destination');
  assert.equal(evalResult.isStale, true);
  assert.equal(evalResult.reason, 'STALE');
});

test('evaluateProximity: marks en-route citizen outside proximity threshold (> 150m)', () => {
  const now = Date.now();
  // 1.5 km away
  const pos: PositionReading = {
    longitude: 76.115,
    latitude: 11.560,
    accuracyMeters: 12,
    timestamp: now,
  };

  const evalResult = evaluateProximity(pos, DEMO_SHELTER, DEFAULT_JOURNEY_OPTIONS, now);
  assert.equal(evalResult.isNear, false);
  assert.equal(evalResult.isStale, false);
  assert.equal(evalResult.isAccurateEnough, true);
  assert.equal(evalResult.reason, 'OUTSIDE_RANGE');
  assert.ok(evalResult.distanceMeters > 1000);
});

test('transitionOnPosition: state changes appropriately between TRACKING and NEAR_DESTINATION', () => {
  // NOT_STARTED ignores position
  const notStarted = transitionOnPosition('NOT_STARTED', {
    distanceMeters: 50,
    isNear: true,
    isStale: false,
    isAccurateEnough: true,
    reason: 'VERIFIED_NEAR',
  });
  assert.equal(notStarted, 'NOT_STARTED');

  // TRACKING transitions to NEAR_DESTINATION when near
  const toNear = transitionOnPosition('TRACKING', {
    distanceMeters: 50,
    isNear: true,
    isStale: false,
    isAccurateEnough: true,
    reason: 'VERIFIED_NEAR',
  });
  assert.equal(toNear, 'NEAR_DESTINATION');

  // Moving away transitions NEAR_DESTINATION back to TRACKING
  const backToTracking = transitionOnPosition('NEAR_DESTINATION', {
    distanceMeters: 500,
    isNear: false,
    isStale: false,
    isAccurateEnough: true,
    reason: 'OUTSIDE_RANGE',
  });
  assert.equal(backToTracking, 'TRACKING');

  // PAUSED ignores position updates
  const paused = transitionOnPosition('PAUSED', {
    distanceMeters: 50,
    isNear: true,
    isStale: false,
    isAccurateEnough: true,
    reason: 'VERIFIED_NEAR',
  });
  assert.equal(paused, 'PAUSED');
});

test('transitionOnArrival: enforces explicit user arrival and strict idempotency', () => {
  // First confirmation succeeds
  const first = transitionOnArrival('NEAR_DESTINATION');
  assert.equal(first.nextState, 'ARRIVAL_REPORTED');
  assert.equal(first.isNewTransition, true);

  // Subsequent/duplicate click is idempotent (no second transition)
  const duplicate = transitionOnArrival('ARRIVAL_REPORTED');
  assert.equal(duplicate.nextState, 'ARRIVAL_REPORTED');
  assert.equal(duplicate.isNewTransition, false);
  assert.equal(duplicate.error, 'ALREADY_REPORTED');

  // Arrival cannot be confirmed if route was revoked
  const revoked = transitionOnArrival('ROUTE_REVOKED');
  assert.equal(revoked.nextState, 'ROUTE_REVOKED');
  assert.equal(revoked.isNewTransition, false);
  assert.equal(revoked.error, 'ROUTE_REVOKED');
});

test('transitionOnRevocation: immediately marks route revoked unless already arrived', () => {
  // Tracking -> Revoked
  assert.equal(transitionOnRevocation('TRACKING'), 'ROUTE_REVOKED');
  assert.equal(transitionOnRevocation('NEAR_DESTINATION'), 'ROUTE_REVOKED');
  assert.equal(transitionOnRevocation('NOT_STARTED'), 'ROUTE_REVOKED');

  // Already arrived stays arrived
  assert.equal(transitionOnRevocation('ARRIVAL_REPORTED'), 'ARRIVAL_REPORTED');
});

test('evaluateArrivalConfirmation: rejects arrival without active stay reservation (fail-closed, no false success)', () => {
  const result = evaluateArrivalConfirmation({
    currentState: 'NEAR_DESTINATION',
    activeStayId: null,
  });

  assert.equal(result.allowed, false);
  assert.equal(result.arrivalSuccess, false);
  assert.equal(result.nextState, 'NEAR_DESTINATION');
  assert.equal(result.canRetry, true);
  assert.ok(result.errorMessage?.includes('No verified stay reservation found'));

  // Also undefined activeStayId
  const undefResult = evaluateArrivalConfirmation({
    currentState: 'TRACKING',
    activeStayId: undefined,
  });
  assert.equal(undefResult.allowed, false);
  assert.equal(undefResult.arrivalSuccess, false);
  assert.equal(undefResult.nextState, 'TRACKING');
});

test('evaluateArrivalConfirmation: rejects arrival when server returns error status or network timeout', () => {
  // Network timeout / lost response
  const timeoutResult = evaluateArrivalConfirmation({
    currentState: 'NEAR_DESTINATION',
    activeStayId: 'stay-123',
    serverResponse: undefined,
  });
  assert.equal(timeoutResult.allowed, false);
  assert.equal(timeoutResult.arrivalSuccess, false);
  assert.equal(timeoutResult.nextState, 'NEAR_DESTINATION');
  assert.equal(timeoutResult.canRetry, true);

  // Server 500 error
  const server500 = evaluateArrivalConfirmation({
    currentState: 'NEAR_DESTINATION',
    activeStayId: 'stay-123',
    serverResponse: { ok: false, status: 500 },
  });
  assert.equal(server500.allowed, false);
  assert.equal(server500.arrivalSuccess, false);
  assert.equal(server500.nextState, 'NEAR_DESTINATION');
  assert.equal(server500.canRetry, true);

  // Server 400 rejection with message
  const server400 = evaluateArrivalConfirmation({
    currentState: 'NEAR_DESTINATION',
    activeStayId: 'stay-123',
    serverResponse: { ok: false, status: 400, error: 'Capacity window closed' },
  });
  assert.equal(server400.allowed, false);
  assert.equal(server400.arrivalSuccess, false);
  assert.equal(server400.errorMessage, 'Capacity window closed');
});

test('evaluateArrivalConfirmation: succeeds only upon valid server acknowledgement and is idempotent', () => {
  const success = evaluateArrivalConfirmation({
    currentState: 'NEAR_DESTINATION',
    activeStayId: 'stay-123',
    serverResponse: { ok: true, status: 200 },
  });
  assert.equal(success.allowed, true);
  assert.equal(success.arrivalSuccess, true);
  assert.equal(success.nextState, 'ARRIVAL_REPORTED');
  assert.equal(success.isNewTransition, true);

  // Idempotent duplicate check
  const duplicate = evaluateArrivalConfirmation({
    currentState: 'ARRIVAL_REPORTED',
    activeStayId: 'stay-123',
    serverResponse: { ok: true, status: 200 },
  });
  assert.equal(duplicate.allowed, false);
  assert.equal(duplicate.arrivalSuccess, true);
  assert.equal(duplicate.isNewTransition, false);
  assert.equal(duplicate.errorMessage, 'ALREADY_REPORTED');
});

test('shouldDropResponse: drops superseded requests and background execution to prevent stale state mutation', () => {
  // Active request matches in foreground -> allowed (not dropped)
  assert.equal(shouldDropResponse(1, 1, false), false);
  assert.equal(shouldDropResponse(42, 42, false), false);

  // Hidden document -> always dropped regardless of request ID
  assert.equal(shouldDropResponse(1, 1, true), true);
  assert.equal(shouldDropResponse(42, 42, true), true);

  // Superseded request ID (newer request was dispatched) -> dropped
  assert.equal(shouldDropResponse(2, 1, false), true);
  assert.equal(shouldDropResponse(10, 9, false), true);

  // Hidden and superseded -> dropped
  assert.equal(shouldDropResponse(2, 1, true), true);
});

test('validateGuidanceSnapshot: validates authoritative package/snapshot and rejects missing/none/stale data', () => {
  // 1. Valid CURRENT guidance with data_version and positive integer snapshot_version
  const valid = validateGuidanceSnapshot('CURRENT', 'PKGDEMO-1:2', 2);
  assert.equal(valid.isValid, true);
  assert.equal(valid.dataVersion, 'PKGDEMO-1:2');
  assert.equal(valid.snapshotVersion, 2);

  // 2. Reject non-CURRENT source status (STALE, UNAVAILABLE, etc.)
  const stale = validateGuidanceSnapshot('STALE', 'PKGDEMO-1:1', 1);
  assert.equal(stale.isValid, false);
  assert.ok(stale.errorMessage?.includes('Source status is STALE'));

  const unavail = validateGuidanceSnapshot(undefined, 'PKGDEMO-1:1', 1);
  assert.equal(unavail.isValid, false);

  // 3. Reject missing or 'none' data_version (no fallback to invented 'v1')
  const noneVer = validateGuidanceSnapshot('CURRENT', 'none', 1);
  assert.equal(noneVer.isValid, false);
  assert.ok(noneVer.errorMessage?.includes('data_version cannot be empty or "none"'));

  const emptyVer = validateGuidanceSnapshot('CURRENT', '', 1);
  assert.equal(emptyVer.isValid, false);

  const undefVer = validateGuidanceSnapshot('CURRENT', undefined, 1);
  assert.equal(undefVer.isValid, false);

  // 4. Reject missing, non-positive, or non-integer snapshot_version
  const zeroSnap = validateGuidanceSnapshot('CURRENT', 'PKGDEMO-1:1', 0);
  assert.equal(zeroSnap.isValid, false);

  const negSnap = validateGuidanceSnapshot('CURRENT', 'PKGDEMO-1:1', -1);
  assert.equal(negSnap.isValid, false);

  const floatSnap = validateGuidanceSnapshot('CURRENT', 'PKGDEMO-1:1', 1.5);
  assert.equal(floatSnap.isValid, false);

  const nullSnap = validateGuidanceSnapshot('CURRENT', 'PKGDEMO-1:1', null);
  assert.equal(nullSnap.isValid, false);
});

test('buildReservationPayload: binds strictly to guidance snapshot_version and fails on invalid/illustrative guidance', () => {
  // 1. Guidance version 2 produces reservation snapshot_version: 2
  const v2Result = buildReservationPayload({
    facilityId: 'FACDEMO-1',
    packageId: 'PKGDEMO-1',
    routeId: 'RTDEMO-1',
    partySize: 2,
    startDate: '2026-09-27',
    endDate: '2026-09-28',
    idempotencyKey: 'idem-123',
    snapshotVersion: 2,
  });
  assert.equal(v2Result.canAllocate, true);
  assert.ok(v2Result.payload);
  assert.equal(v2Result.payload.snapshot_version, 2);
  assert.equal(v2Result.payload.facility_id, 'FACDEMO-1');
  assert.equal(v2Result.payload.route_id, 'RTDEMO-1');

  // 2. Missing, non-positive, or null snapshot version blocks reservation
  const nullSnap = buildReservationPayload({
    facilityId: 'FACDEMO-1',
    packageId: 'PKGDEMO-1',
    routeId: 'RTDEMO-1',
    partySize: 1,
    startDate: '2026-09-27',
    endDate: '2026-09-28',
    idempotencyKey: 'idem-123',
    snapshotVersion: null,
  });
  assert.equal(nullSnap.canAllocate, false);
  assert.ok(nullSnap.errorMessage?.includes('Missing or non-positive snapshot version'));

  // 3. Illustrative guidance blocks reservation
  const illus = buildReservationPayload({
    facilityId: 'FACDEMO-1',
    packageId: 'PKGDEMO-1',
    routeId: 'RTDEMO-1',
    partySize: 1,
    startDate: '2026-09-27',
    endDate: '2026-09-28',
    idempotencyKey: 'idem-123',
    snapshotVersion: 1,
    isIllustrative: true,
  });
  assert.equal(illus.canAllocate, false);
  assert.ok(illus.errorMessage?.includes('Cannot allocate reservation against illustrative preview'));
});

test('buildReservationPayload & mapGuidanceDestinations: preserves missing route identity without demo fallback', () => {
  // When guidance provides no route_id, leave it undefined and omit from payload
  const rawList: RawGuidanceDestination[] = [
    {
      facility_id: 'FACDEMO-1',
      safe_zone_id: 'SZDEMO-1',
      capacity_known: true,
      free: 10,
      route_id: undefined, // Absent from server
      route_verified: false,
    },
    {
      facility_id: 'FACDEMO-2',
      safe_zone_id: 'SZDEMO-2',
      capacity_known: true,
      free: 5,
      route_id: 'RTDEMO-2',
      route_verified: true,
    },
  ];

  const mapped = mapGuidanceDestinations(rawList, 'FACDEMO-1', [76.105, 11.570]);
  assert.equal(mapped[0].route_id, undefined, 'Absent route_id must remain undefined, not defaulted to demo route');
  assert.equal(mapped[1].route_id, 'RTDEMO-2');

  // Payload without routeId omits route_id
  const noRouteRes = buildReservationPayload({
    facilityId: mapped[0].facility_id,
    packageId: 'PKGDEMO-1',
    routeId: mapped[0].route_id,
    partySize: 1,
    startDate: '2026-09-27',
    endDate: '2026-09-28',
    idempotencyKey: 'idem-no-route',
    snapshotVersion: 1,
  });
  assert.equal(noRouteRes.canAllocate, true);
  assert.ok(noRouteRes.payload);
  assert.equal('route_id' in noRouteRes.payload, false, 'Payload must omit route_id when server provided none');

  // Payload with verified routeId includes route_id
  const withRouteRes = buildReservationPayload({
    facilityId: mapped[1].facility_id,
    packageId: 'PKGDEMO-1',
    routeId: mapped[1].route_id,
    partySize: 1,
    startDate: '2026-09-27',
    endDate: '2026-09-28',
    idempotencyKey: 'idem-with-route',
    snapshotVersion: 1,
  });
  assert.equal(withRouteRes.canAllocate, true);
  assert.ok(withRouteRes.payload);
  assert.equal(withRouteRes.payload.route_id, 'RTDEMO-2');
});

test('mapGuidanceDestinations: does not assign demo coordinates or distance to other facilities', () => {
  const rawList: RawGuidanceDestination[] = [
    {
      facility_id: 'FACDEMO-1',
      safe_zone_id: 'SZDEMO-1',
      capacity_known: true,
      free: 10,
      route_verified: true,
    },
    {
      facility_id: 'FACDEMO-2',
      safe_zone_id: 'SZDEMO-2',
      capacity_known: true,
      free: 5,
      route_verified: false,
    },
  ];

  const demoCoords: [number, number] = [76.105, 11.570];
  const userPos = { longitude: 76.123, latitude: 11.553 };

  const mapped = mapGuidanceDestinations(rawList, 'FACDEMO-1', demoCoords, userPos);

  // Demo facility gets coordinates and distance
  assert.deepEqual(mapped[0].coordinates, demoCoords);
  assert.ok(typeof mapped[0].distance_km === 'number');

  // Alternate facility must NOT get demo facility coordinates or distance
  assert.equal(mapped[1].coordinates, undefined);
  assert.equal(mapped[1].distance_km, undefined);
});

test('resolveChoiceAgainstGuidance: resolves against verified destinations and rejects unresolved without invented identity', () => {
  const verifiedList: ResolvedDestinationChoice[] = [
    {
      facility_id: 'FACDEMO-1',
      safe_zone_id: 'SZDEMO-1',
      facility_name: 'Demo Safe Facility (SZDEMO-1)',
      capacity_known: true,
      free: 10,
      route_verified: true,
      is_illustrative: false,
    },
    {
      facility_id: 'FACDEMO-2',
      safe_zone_id: 'SZDEMO-2',
      facility_name: 'Alternate Facility (Ward 10)',
      capacity_known: true,
      free: 4,
      route_verified: false,
      is_illustrative: false,
    },
    {
      facility_id: 'FACDEMO-PREVIEW',
      safe_zone_id: 'SZDEMO-PREVIEW',
      facility_name: 'Preview Facility',
      capacity_known: false,
      free: null,
      route_verified: false,
      is_illustrative: true,
    },
  ];

  // 1. Matches verified facility
  const match1 = resolveChoiceAgainstGuidance('FACDEMO-1', verifiedList);
  assert.equal(match1.resolved, true);
  assert.equal(match1.destination?.facility_id, 'FACDEMO-1');
  assert.equal(match1.destination?.safe_zone_id, 'SZDEMO-1');

  // 2. Unresolved facility is rejected and does not fabricate safe_zone_id or route
  const unverified = resolveChoiceAgainstGuidance('FAC-UNKNOWN-99', verifiedList);
  assert.equal(unverified.resolved, false);
  assert.equal(unverified.destination, null);
  assert.ok(unverified.errorMessage?.includes('FAC-UNKNOWN-99'));

  // 3. Illustrative destination is not resolved as verified
  const previewMatch = resolveChoiceAgainstGuidance('FACDEMO-PREVIEW', verifiedList);
  assert.equal(previewMatch.resolved, false);
  assert.equal(previewMatch.destination, null);
});

