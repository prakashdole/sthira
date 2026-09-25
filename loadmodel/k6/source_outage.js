// Source outage scenario. Models a source-upstream outage: 100% of
// /api/v3/packages/{id}/versions/{v} responses return 503 DATA_UNAVAILABLE.
// The synthetic fixture flips the outage rate via the test operator (see
// run_scenario.sh). Other routes remain healthy.
//
// This is the fail-closed path: clients must see the error class quickly
// and not retry-storm the upstream. The harness measures p95/p99 latency
// on the 503 path and asserts it's bounded.
import http from 'k6/http';
import { check } from 'k6';
import {
  DEFAULT_THRESHOLDS,
  standardRequestParams,
} from './lib/options.js';

export const options = {
  scenarios: {
    outage: {
      executor: 'constant-arrival-rate',
      rate: 100,
      timeUnit: '1s',
      duration: '60s',
      preAllocatedVUs: 20,
      maxVUs: 50,
      exec: 'outageScenario',
    },
  },
  thresholds: {
    // The 503 must come back fast. Bounding p95 to 200 ms proves the
    // handler short-circuits the upstream call.
    http_req_failed:   ['rate>0.5'], // 503 IS expected — the threshold expects the failure rate
    http_req_duration: ['p(95)<200', 'p(99)<500'],
  },
};

export function outageScenario() {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  const params = standardRequestParams();
  const res = http.get(
    `${baseURL}/api/v3/packages/PKG-KL-WAYANAD-01/versions/1`,
    params,
  );
  check(res, { '503 (expected)': (r) => r.status === 503 });
}
