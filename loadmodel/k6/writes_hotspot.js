// Writes scenario. Drives /api/v3/reservations at 50 writes/s, with 80%
// targeting one facility (one-facility hotspot). The synthetic store
// reserves capacity=1 per facility, so concurrent writers after the first
// will hit CAPACITY_CONFLICT — that's the expected fail-closed behaviour
// the orchestrator + store must implement (D28, D49).
import http from 'k6/http';
import { check } from 'k6';
import {
  WRITE_THRESHOLDS,
  standardRequestParams,
  reservationPayload,
  uniqueIdempotencyKey,
} from './lib/options.js';

export const options = {
  scenarios: {
    hotspot_50: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: '60s',
      preAllocatedVUs: 30,
      maxVUs: 50,
      exec: 'writeHotspotScenario',
    },
  },
  thresholds: WRITE_THRESHOLDS,
};

const FACILITIES = [
  'FACILITY-DEMO-1', 'FACILITY-DEMO-2', 'FACILITY-DEMO-3',
  'FACILITY-DEMO-4', 'FACILITY-DEMO-5', 'FACILITY-DEMO-6',
  'FACILITY-DEMO-7', 'FACILITY-DEMO-8', 'FACILITY-DEMO-9',
  'FACILITY-DEMO-10',
];

export function writeHotspotScenario() {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  // 80% of writes target the hotspot. The other 20% spread across the
  // remaining 9 facilities. Capacity is 1 each, so even at 20% off-hotspot
  // we expect some CAPACITY_CONFLICT responses on round-robin overflow.
  const r = Math.random();
  const facilityID = r < 0.8 ? FACILITIES[0] : FACILITIES[1 + Math.floor(Math.random() * (FACILITIES.length - 1))];
  const body = reservationPayload(facilityID, uniqueIdempotencyKey());
  const params = standardRequestParams();
  const res = http.post(`${baseURL}/api/v3/reservations`, body, params);
  // Capacity conflict is the expected outcome under contention. It's a
  // success for the harness: the fail-closed path is exercised.
  if (res.status === 409) {
    check(res, { 'capacity conflict (expected)': (r2) => r2.status === 409 });
    return;
  }
  check(res, { 'created': (r2) => r2.status === 201 });
}
