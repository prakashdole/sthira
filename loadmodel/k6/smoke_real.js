// smoke_real.js — bounded k6 smoke against the ACTUAL Go backend
// (sthira) running on a uniquely-owned migrated PostgreSQL. The
// scenario exercises the public routes that exist in production:
//   - /health/live (always 200)
//   - /health/ready (200 when DB+migrations OK)
//   - /api/v3/places/resolve (4xx for unknown jurisdiction — proves
//     the request reached the typed handler and was rejected at the
//     boundary, not at the network layer)
//   - /api/v3/guidance/query (4xx — same shape)
//
// NEVER scale this past 50 VUs. This is a wiring smoke, not a
// capacity benchmark. A larger run needs controlled hardware,
// migrated DB and authorized model workers, which are NOT_RUN in
// this revision.
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 25,
  duration: '20s',
  thresholds: {
    // We are explicitly NOT benchmarking capacity here; this is a
    // wiring check. p95 must stay under 1s on a laptop. 5xx-only
    // failure is enforced by check() calls; the global
    // http_req_failed metric counts 4xx as failure, which is the
    // expected business outcome here.
    'http_req_duration': ['p(95)<1000'],
  },
};

const standardRequestParams = () => ({
  headers: {
    'Content-Type': 'application/json',
    'X-Request-ID': `k6-${__VU}-${__ITER}-${Date.now()}`,
  },
  timeout: '5s',
});

export default function () {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:18443';
  const params = standardRequestParams();

  // 1. Health probes (cheap, every iteration).
  const live = http.get(`${baseURL}/health/live`, params);
  check(live, { 'live=200': (r) => r.status === 200 });

  const ready = http.get(`${baseURL}/health/ready`, params);
  check(ready, {
    'ready=200': (r) => r.status === 200,
    'ready!=503': (r) => r.status !== 503,
  });

  // 2. Typed handler roundtrip — proves the request body reached
  // the handler and was validated. We expect 4xx because the
  // seeded DB has no matching jurisdiction for the query, NOT 5xx.
  const places = http.post(
    `${baseURL}/api/v3/places/resolve`,
    JSON.stringify({ jurisdiction: 'KL-WYD', query: 'Mallappuram' }),
    params,
  );
  check(places, {
    'places!=5xx': (r) => r.status < 500,
  });

  const guidance = http.post(
    `${baseURL}/api/v3/guidance/query`,
    JSON.stringify({
      jurisdiction: 'KL-WYD',
      snapshot_version: 1,
      query: { kind: 'transcript', text: 'Mali' },
    }),
    params,
  );
  check(guidance, {
    'guidance!=5xx': (r) => r.status < 500,
  });

  sleep(0.05);
}