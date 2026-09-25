// smoke_business.js — bounded k6 smoke that exercises the ACTUAL
// sthira Go binary's SEEDED business flow: a place resolve that
// returns the exact seeded candidate, asserted SEMANTICALLY.
//
// The harness (scripts/run_k6_smoke_real.sh --business-flow) seeds
// exactly one isolated place_aliases row in its own uniquely-named
// database and exports its identity here. Assertions check the
// envelope contract (schema_version "3.0", non-empty request_id)
// and the typed payload (data.place_id / data.place_kind equal the
// seeded values) — never body length, which passed on error and
// empty-payload 200s alike.
//
// Failure taxonomy (all numeric thresholds, so a broken run fails
// the script rather than silently passing):
//   status 0        -> connection failure (server down mid-run)
//   5xx             -> handler/DB failure
//   4xx on the      -> flow regression (the seeded row must exist;
//   seeded query       a 404 here means seeding/schema/case drift)
//   empty/malformed -> semantic failure (200 but not the contract)
//   body present but -> semantic failure (wrong place identity)
//   wrong candidate
//
// NEVER scale past 50 VUs. Larger loads need controlled hardware,
// which is NOT_RUN in this revision.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const failCounter = new Counter('business_flow_failures');
const connectFailures = new Rate('business_connect_failures');
const httpFailures = new Counter('business_http_5xx');
const semanticFailures = new Counter('business_semantic_failures');

export const options = {
  vus: 10,
  duration: '15s',
  thresholds: {
    'business_connect_failures': ['rate==0'],
    'business_http_5xx': ['count==0'],
    'business_flow_failures': ['count==0'],
    'business_semantic_failures': ['count==0'],
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
  const jurisdiction = __ENV.SMOKE_BUSINESS_JURISDICTION || '';
  const placeQuery = __ENV.SMOKE_BUSINESS_QUERY || '';
  const wantPlaceID = __ENV.SMOKE_BUSINESS_PLACE_ID || '';
  const wantPlaceKind = __ENV.SMOKE_BUSINESS_PLACE_KIND || 'FACILITY';
  if (!jurisdiction || !placeQuery || !wantPlaceID) {
    // Running this script without the harness seed environment is a
    // configuration error, not a pass.
    failCounter.add(1);
    semanticFailures.add(1);
    return;
  }
  const params = standardRequestParams();

  // 1. Health probes — cheap and every iteration.
  const live = http.get(`${baseURL}/health/live`, params);
  check(live, { 'live=200': (r) => r.status === 200 });
  if (live.status === 0) { connectFailures.add(1); failCounter.add(1); }
  if (live.status >= 500) { httpFailures.add(1); failCounter.add(1); }

  const ready = http.get(`${baseURL}/health/ready`, params);
  check(ready, { 'ready=200': (r) => r.status === 200 });
  if (ready.status === 0) { connectFailures.add(1); failCounter.add(1); }
  if (ready.status >= 500) { httpFailures.add(1); failCounter.add(1); }

  // 2. Seeded business flow: resolve MUST return the exact
  // candidate this run seeded. 404/409/400 all count as
  // regressions here because the seed is isolated and present.
  const resolve = http.post(
    `${baseURL}/api/v3/places/resolve`,
    JSON.stringify({ jurisdiction, query: placeQuery }),
    params,
  );
  const ok = resolve.status === 200;
  let env = null;
  if (ok) {
    try { env = JSON.parse(resolve.body); } catch (_) { env = null; }
  }
  const shapeOk = !!env
    && typeof env.request_id === 'string' && env.request_id.length > 0
    && env.schema_version === '3.0'
    && !!env.data
    && env.data.place_id === wantPlaceID
    && env.data.place_kind === wantPlaceKind;

  check(resolve, {
    'resolve=200': () => ok,
    'resolve=envelope-contract': () => shapeOk,
    'resolve=candidate-identity': () => !!env && !!env.data && env.data.place_id === wantPlaceID,
  });

  if (resolve.status === 0) { connectFailures.add(1); failCounter.add(1); }
  if (resolve.status >= 500) { httpFailures.add(1); failCounter.add(1); }
  if (!ok) { failCounter.add(1); }
  // A 200 that is empty, malformed, or carries the wrong
  // candidate is a semantic failure — never a pass.
  if (ok && !shapeOk) { semanticFailures.add(1); failCounter.add(1); }

  sleep(0.05);
}
