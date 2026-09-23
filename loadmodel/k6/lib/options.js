// loadmodel/k6/lib/options.js
// Shared options, payloads, and envelope helpers for P7 k6 load scenarios.

import { check } from 'k6';

export const DEFAULT_THRESHOLDS = {
  http_req_failed: ['rate<0.01'],
  http_req_duration: ['p(95)<500', 'p(99)<1000'],
};

export const WRITE_THRESHOLDS = {
  http_req_duration: ['p(95)<1000', 'p(99)<2000'],
};

export const VOICE_THRESHOLDS = {
  http_req_duration: ['p(95)<8000', 'p(99)<8000'],
};

export function standardRequestParams(overrides = {}) {
  const reqId = `k6-${Date.now()}-${Math.floor(Math.random() * 100000)}`;
  const defaultHeaders = {
    'Content-Type': 'application/json',
    'X-Request-ID': reqId,
    'X-Jurisdiction': 'KL-WYD',
  };
  return {
    headers: Object.assign({}, defaultHeaders, overrides.headers || {}),
    timeout: overrides.timeout || '10s',
  };
}

export function cachedRead(baseURL, kind) {
  switch (kind) {
    case 'manifest':
      return { method: 'GET', url: `${baseURL}/api/v3/regions/KL-WYD/manifest` };
    case 'card':
      return { method: 'GET', url: `${baseURL}/api/v3/packages/PKG-DEMO/versions/v1` };
    case 'resource':
      return { method: 'GET', url: `${baseURL}/api/v3/resources/RES-DEMO` };
    default:
      return { method: 'GET', url: `${baseURL}/api/v3/regions/KL-WYD/manifest` };
  }
}

export function placeResolvePayload() {
  return JSON.stringify({
    query: 'Meppadi',
    jurisdiction: 'KL',
  });
}

export function guidanceQueryPayload() {
  return JSON.stringify({
    origin_coordinates: [76.13, 11.55],
    jurisdiction: 'KL-WYD',
  });
}

export function reservationPayload(facilityID, idempotencyKey) {
  return JSON.stringify({
    facility_id: facilityID || 'FACILITY-DEMO-1',
    party_size: 1,
    service_date: '2026-09-23',
    idempotency_key: idempotencyKey || uniqueIdempotencyKey(),
  });
}

export function voiceProcessPayload(includeTTS = false) {
  return JSON.stringify({
    audio_bytes: 'UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=',
    language: 'ml-IN',
    render: {
      kind: includeTTS ? 'tts' : 'text',
    },
  });
}

export function uniqueIdempotencyKey() {
  return `IDEM-${Date.now()}-${Math.floor(Math.random() * 1000000)}`;
}

export function assertValidEnvelope(res, label) {
  check(res, {
    [`${label} status ok`]: (r) => r.status >= 200 && r.status < 400,
    [`${label} envelope valid`]: (r) => {
      const ct = (r.headers['Content-Type'] || r.headers['content-type'] || '');
      if (ct.includes('application/json')) {
        try {
          const body = JSON.parse(r.body);
          return body && ('data' in body || 'status' in body || 'errors' in body);
        } catch (e) {
          return false;
        }
      }
      return r.body && r.body.length > 0;
    },
  });
}
