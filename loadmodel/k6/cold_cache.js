// Cold-cache scenario. Tests the cold-origin path: the synthetic fixture's
// cache-hit rate is flipped to 0% via the test operator (see run_scenario.sh),
// so 100% of public reads hit the origin. This models the cold-start case
// from WORKLOAD.md §2.
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
    cold_1k: {
      executor: 'constant-arrival-rate',
      rate: 1000,
      timeUnit: '1s',
      // 30s cold burst + 60s warm tail. The burst overwhelms the origin;
      // the warm tail exercises the post-recovery path.
      duration: '90s',
      startTime: '0s',
      preAllocatedVUs: 50,
      maxVUs: 100,
      exec: 'coldReadScenario',
    },
  },
  thresholds: DEFAULT_THRESHOLDS,
};

const KINDS = ['manifest', 'card', 'resource'];

export function coldReadScenario() {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  const params = standardRequestParams();
  const kind = KINDS[Math.floor(Math.random() * KINDS.length)];
  const r = cachedRead(baseURL, kind);
  const res = http.request(r.method, r.url, null, params);
  assertValidEnvelope(res, `cold.${kind}`);
  check(res, { '200': (r2) => r2.status === 200 });
}
