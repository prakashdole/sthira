/**
 * Lost-Reservation Retry — Pure Logic Regression
 *
 * Tests the core retry contract for the reservation flow:
 * - Network loss (status=null) → keepPendingReservation=true → pending kept → same idempotency key
 * - 5xx server error → keepPendingReservation=true → pending kept → same idempotency key
 * - 409 IDEMPOTENCY_CONFLICT + retryable=false → keepPendingReservation=false → pending cleared
 * - 409 IDEMPOTENCY_CONFLICT + retryable=true → keepPendingReservation=true → pending kept
 *
 * Run: cd frontend/v2/src && node --experimental-strip-types --test reservation_retry_logic.test.ts
 * Status: NOT_RUN — deferred until after usage reset
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { keepPendingReservation } from './journey.ts';

/**
 * Observable acceptance condition:
 * After a network-error reservation submission, retry MUST use the same
 * idempotency_key stored in sessionStorage (pending_reservation), and the
 * server's idempotency replay MUST return the SAME stay_id without creating a second one.
 *
 * Failure mode (before fix):
 * If keepPendingReservation incorrectly returns false for status=null (network error),
 * the pending_reservation would be cleared and a fresh idempotency key would be generated,
 * causing the server to treat it as a NEW request — potentially creating a duplicate stay.
 */

test('keepPendingReservation: network loss (status=null) keeps pending — enables safe retry', () => {
  // Simulates: browser submitted POST /api/v3/reservations, request reached server,
  // server processed it, but response was lost (network error / tab crash).
  const result = keepPendingReservation(null, null);
  assert.equal(
    result,
    true,
    'Network-loss status=null MUST keep pending so retry reuses the same idempotency key. ' +
    'If this fails, a second submitReservation call would generate a NEW idempotency key, ' +
    'causing the server to INSERT a second stay instead of replaying the first.'
  );
});

test('keepPendingReservation: 5xx keeps pending — server processed but response failed', () => {
  assert.equal(keepPendingReservation(500, null), true, '500 => keep pending');
  assert.equal(keepPendingReservation(502, { errors: [{ code: 'BAD_GATEWAY', retryable: false }] }), true, '502 => keep pending regardless of retryable flag');
  assert.equal(keepPendingReservation(503, null), true, '503 => keep pending');
  assert.equal(keepPendingReservation(504, null), true, '504 => keep pending');
});

test('keepPendingReservation: 409 IDEMPOTENCY_CONFLICT with retryable=true keeps pending', () => {
  // Simulates: server returned 409 because another request with same idempotency key
  // is still in-flight (not yet committed). Client should retry.
  const err = { errors: [{ code: 'IDEMPOTENCY_CONFLICT', retryable: true, message: 'request in progress' }] };
  assert.equal(keepPendingReservation(409, err), true);
});

test('keepPendingReservation: 409 IDEMPOTENCY_CONFLICT with retryable=false drops pending', () => {
  // Simulates: server returned 409 because the payload changed (same key, different body).
  // Client should NOT retry with same key — must generate a new key for a fresh attempt.
  const err = { errors: [{ code: 'IDEMPOTENCY_CONFLICT', retryable: false, message: 'payload mismatch' }] };
  assert.equal(
    keepPendingReservation(409, err),
    false,
    '409 IDEMPOTENCY_CONFLICT with retryable=false means server rejected the request permanently. ' +
    'Clearing pending lets the user initiate a fresh request with a new idempotency key. ' +
    'If this incorrectly returns true, the retry would send the SAME conflicting key again.'
  );
});

test('keepPendingReservation: 404/400/401/403 definitive errors drop pending', () => {
  assert.equal(keepPendingReservation(404, { errors: [{ code: 'NOT_FOUND', retryable: false }] }), false);
  assert.equal(keepPendingReservation(400, { errors: [{ code: 'BAD_REQUEST', retryable: false }] }), false);
  assert.equal(keepPendingReservation(401, null), false);
  assert.equal(keepPendingReservation(403, null), false);
});

test('keepPendingReservation: 409 non-idempotency errors drop pending', () => {
  // STALE_VERSION means the guidance snapshot changed — user must get fresh guidance
  const staleErr = { errors: [{ code: 'STALE_VERSION', retryable: true }] };
  assert.equal(keepPendingReservation(409, staleErr), false, 'STALE_VERSION should drop pending');

  // CAPACITY_CONFLICT means no space — retrying with same key won't help
  const capErr = { errors: [{ code: 'CAPACITY_CONFLICT', retryable: true }] };
  assert.equal(keepPendingReservation(409, capErr), false, 'CAPACITY_CONFLICT should drop pending');
});

test('keepPendingReservation: 200 success would not be called through this path', () => {
  // keepPendingReservation is only called in the non-ok path (submitReservation line 892).
  // A 200 result clears pending immediately (line 862: sessionStorage.removeItem).
  // This test documents the expected behavior if the function is ever misused for a 200.
  const result = keepPendingReservation(200, null);
  assert.equal(result, false, '200 should not keep pending (not called in practice)');
});
