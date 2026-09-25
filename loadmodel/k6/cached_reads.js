// Cached public reads. Models warm-edge-cache behaviour at 95% hit rate.
//
// VUs are chosen so the k6 process itself is NOT the bottleneck: 50 VUs
// can sustain ~1K QPS on a modern laptop. The synthetic fixture has the
// 95% hit rate baked in.
//
// This scenario is the cached-public-delivery boundary test. It does NOT
// mix in voice or writes; those have their own scenarios.
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
    cached_1k: {
      executor: 'constant-arrival-rate',
      rate: 1000,
      timeUnit: '1s',
      duration: '60s',
      preAllocatedVUs: 50,
      maxVUs: 100,
      exec: 'cachedReadScenario',
    },
  },
  thresholds: DEFAULT_THRESHOLDS,
};

const KINDS = ['manifest', 'card', 'resource'];

export function cachedReadScenario() {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  const params = standardRequestParams();
  const kind = KINDS[Math.floor(Math.random() * KINDS.length)];
  const r = cachedRead(baseURL, kind);
  const res = http.request(r.method, r.url, null, params);
  assertValidEnvelope(res, `cached.${kind}`);
  check(res, { '200': (r2) => r2.status === 200 });
}
