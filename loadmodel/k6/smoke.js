// Smoke scenario. First scenario to run; catches wiring bugs.
// VUs: 25, duration: 30s. Mixes all route classes so wiring problems show
// up early. NEVER scale this scenario past 50 VUs.
import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  DEFAULT_THRESHOLDS,
  standardRequestParams,
  placeResolvePayload,
  guidanceQueryPayload,
  cachedRead,
  assertValidEnvelope,
} from './lib/options.js';

export const options = {
  vus: 25,
  duration: '30s',
  thresholds: DEFAULT_THRESHOLDS,
};

export default function () {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  const params = standardRequestParams();

  // 60% cached reads, 30% public reads, 10% voice. The mix is illustrative;
  // individual scenarios pin specific mixes.
  const roll = Math.random();
  if (roll < 0.6) {
    const r = cachedRead(baseURL, 'manifest');
    const res = http.request(r.method, r.url, null, params);
    assertValidEnvelope(res, 'smoke.manifest');
    check(res, { '200': (r2) => r2.status === 200 });
  } else if (roll < 0.9) {
    const body = placeResolvePayload();
    const res = http.post(`${baseURL}/api/v3/places/resolve`, body, params);
    assertValidEnvelope(res, 'smoke.places');
    check(res, { '200': (r2) => r2.status === 200 });
  } else {
    const body = guidanceQueryPayload();
    const res = http.post(`${baseURL}/api/v3/guidance/query`, body, params);
    assertValidEnvelope(res, 'smoke.guidance');
    check(res, { '200': (r2) => r2.status === 200 });
  }
  sleep(0.05);
}
