import test from 'node:test';
import assert from 'node:assert/strict';
import {
  computeDistanceMeters,
  evaluateProximity,
  transitionOnPosition,
  transitionOnArrival,
  transitionOnRevocation,
  DEFAULT_JOURNEY_OPTIONS,
  type DestinationTarget,
  type PositionReading,
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
