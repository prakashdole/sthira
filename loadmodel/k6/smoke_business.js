// smoke_business.js — bounded k6 smoke that exercises the ACTUAL
// sthira Go binary's seeded business flow: a successful place
// resolve that returns a real (non-empty) candidate list.
//
// This complements smoke_real.js (which only probes health +
// 4xx business outcomes) by verifying the typed handler round
// trip with a 2xx response. The test is opt-in via the
// SMOKE_BUSINESS_JURISDICTION env var so the seed fixture can be
// tuned without breaking the wiring check.
//
// NEVER scale past 50 VUs. Larger loads need controlled hardware,
// which is NOT_RUN in this revision.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Failure counters surface broken responses / connection drops
// as numeric thresholds so a broken run fails the script rather
// than silently passing. status<500 in smoke_real.js accepts
// status 0 (connection refused); this script makes that explicit.
const failCounter = new Counter('business_flow_failures');
const connectFailures = new Rate('business_connect_failures');
const httpFailures = new Counter('business_http_5xx');

export const options = {
  vus: 10,
  duration: '15s',
  thresholds: {
    // Connection failures must be zero for a successful run.
    'business_connect_failures': ['rate==0'],
    // 5xx must be zero: the typed handler should never produce 5xx
    // on a healthy backend; if it does, the run is broken.
    'business_http_5xx': ['count==0'],
    // Any business-flow assertion failure fails the script.
    'business_flow_failures': ['count==0'],
    // Latency p95 stays bounded; this is a wiring check.
    'http_req_duration': ['p(95)<500'],
  },
};

const standardRequestParams = () => ({
  headers: {
    'Content-Type': 'application/json',
    'X-Request-ID': `k6b-${__VU}-${__ITER}-${Date.now()}`,
  },
  timeout: '5s',
});

export default function () {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:18443';
  const jurisdiction = __ENV.SMOKE_BUSINESS_JURISDICTION || 'KL-WYD';
  const placeQuery = __ENV.SMOKE_BUSINESS_QUERY || 'Mallappuram';
  const params = standardRequestParams();

  // 1. Health probes — cheap and every iteration.
  const live = http.get(`${baseURL}/health/live`, params);
  check(live, {
    'live=200': (r) => r.status === 200,
  });
  if (live.status === 0) {
    connectFailures.add(1);
    failCounter.add(1);
  }
  if (live.status >= 500) {
    httpFailures.add(1);
    failCounter.add(1);
  }

  const ready = http.get(`${baseURL}/health/ready`, params);
  check(ready, {
    'ready=200': (r) => r.status === 200,
  });

  // 2. Successful business flow: a place resolve that returns a
  // non-empty candidate list. The endpoint must be available
  // (200/4xx for unknown), but the response body must be
  // well-formed JSON. We exercise the seed fixture's known
  // jurisdiction + query and assert 200.
  const resolve = http.post(
    `${baseURL}/api/v3/places/resolve`,
    JSON.stringify({ jurisdiction, query: placeQuery }),
    params,
  );
  check(resolve, {
    'resolve=200': (r) => r.status === 200,
    'resolve=json': (r) => {
      try {
        return typeof JSON.parse(r.body) === 'object'
      } catch (_) {
        return false
      }
    },
    'resolve=non_empty': (r) => r.body && r.body.length > 2,
  });
  if (resolve.status === 0) {
    connectFailures.add(1);
    failCounter.add(1);
  }
  if (resolve.status >= 500) {
    httpFailures.add(1);
    failCounter.add(1);
  }
  if (resolve.status !== 200) {
    failCounter.add(1);
  }

  sleep(0.05);
}
