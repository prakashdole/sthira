// Burst scenario. Synchronised arrival burst: a 30 s burst at 3K QPS,
// then drain. Models the alert-broadcast or evacuation-notice push case
// where many users refresh at the same time.
//
// VUs are pre-allocated so the burst doesn't wait for VU spawn. The
// scenario size is bounded by what a single laptop can sustain: 100 VUs
// at ~30 rps each = 3K QPS.
import http from 'k6/http';
import { check } from 'k6';
import {
  DEFAULT_THRESHOLDS,
  standardRequestParams,
  cachedRead,
  assertValidEnvelope,
} from './lib/options.js';

export const options = {
  scenarios: {
    burst: {
      executor: 'constant-arrival-rate',
      rate: 3000,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 100,
      maxVUs: 150,
      exec: 'burstScenario',
    },
    drain: {
      executor: 'constant-arrival-rate',
      rate: 500,
      timeUnit: '1s',
      duration: '30s',
      startTime: '30s',
      preAllocatedVUs: 30,
      maxVUs: 50,
      exec: 'burstScenario',
    },
  },
  thresholds: DEFAULT_THRESHOLDS,
};

const KINDS = ['manifest', 'card', 'resource'];

export function burstScenario() {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  const params = standardRequestParams();
  const kind = KINDS[Math.floor(Math.random() * KINDS.length)];
  const r = cachedRead(baseURL, kind);
  const res = http.request(r.method, r.url, null, params);
  assertValidEnvelope(res, `burst.${kind}`);
  check(res, { '200': (r2) => r2.status === 200 });
}
