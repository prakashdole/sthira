// smoke_real.js — bounded k6 smoke against the ACTUAL sthira Go
// binary running on a uniquely-owned migrated PostgreSQL. The
// scenario exercises the public routes that exist in production:
//
//   - /health/live (always 200)
//   - /health/ready (200 when DB+migrations OK)
//   - /api/v3/places/resolve (4xx for unknown jurisdiction — proves
//     the request reached the typed handler and was rejected at
//     the boundary, not at the network layer)
//   - /api/v3/guidance/query (4xx — same shape)
//
// The smoke distinguishes:
//   - 5xx: a real failure (the handler crashed, the DB is down)
//   - 4xx: the expected business outcome (empty seeded jurisdiction)
//   - 0 (connection refused): a startup failure (the binary never
//     came up, or it died during the run)
//
// All three categories are reported as numeric metrics and the
// script FAILS if any of them are non-zero. The previous version
// silently accepted 0 as < 500, which made startup failures look
// like a successful run.
//
// NEVER scale past 50 VUs. Larger loads need controlled hardware,
// which is NOT_RUN in this revision.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate } from 'k6/metrics';

const connectFailures = new Rate('connect_failures');
const http5xx = new Counter('http_5xx_failures');
const http4xxOk = new Counter('http_4xx_expected');

export const options = {
  vus: 25,
  duration: '20s',
  thresholds: {
    // A startup failure (binary down, port collision, etc.) MUST
    // be reported as a failure, not silently absorbed by the
    // status<500 check.
    'connect_failures': ['rate==0'],
    // Any 5xx response is a real failure.
    'http_5xx_failures': ['count==0'],
    // Latency p95 must stay under 1s on a laptop.
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
  check(live, {
    'live=200': (r) => r.status === 200,
  });
  if (live.status === 0) {
    connectFailures.add(1);
  }
  if (live.status >= 500) {
    http5xx.add(1);
  }

  const ready = http.get(`${baseURL}/health/ready`, params);
  check(ready, {
    'ready=200': (r) => r.status === 200,
    'ready!=503': (r) => r.status !== 503,
  });
  if (ready.status === 0) {
    connectFailures.add(1);
  }
  if (ready.status >= 500) {
    http5xx.add(1);
  }

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
  if (places.status === 0) {
    connectFailures.add(1);
  }
  if (places.status >= 500) {
    http5xx.add(1);
  }
  if (places.status >= 400 && places.status < 500) {
    http4xxOk.add(1);
  }

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
  if (guidance.status === 0) {
    connectFailures.add(1);
  }
  if (guidance.status >= 500) {
    http5xx.add(1);
  }
  if (guidance.status >= 400 && guidance.status < 500) {
    http4xxOk.add(1);
  }

  sleep(0.05);
}
