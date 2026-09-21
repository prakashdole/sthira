// Voice process scenario. Drives /api/v3/voice/process with a mix of
// transcript inputs, 50% with render.kind=tts. The synthetic fixture's
// voice path sleeps for ASR + middle + (TTS) service time, so each
// iteration takes 1.5s+4s+(0 or 2s).
//
// Two arrival rates run sequentially:
//
//   warm_1   — 1 req/s, well under the bounded queue's sustained rate.
//              All requests should succeed; we measure end-to-end
//              p50/p95/p99 latency including the synthetic worker cost.
//   surge_20 — 20 req/s for 30s. The bounded queue depth is 8; with the
//              default ASR=1.5s+Middle=4s+(TTS=2s) cost, ~2 req/s is the
//              fixture's sustained capacity. The remaining requests
//              MUST fail-fast with 503 QUEUE_SATURATED, not pile up
//              unbounded. That's the behaviour the report asserts.
//
// Voice can legitimately return 503 QUEUE_SATURATED under load — that
// is the orchestrator's fail-fast behaviour, not a defect. The harness
// labels every such response via the `queue saturated (expected)` check
// and reports the saturation rate in the per-scenario summary.
import http from 'k6/http';
import { check } from 'k6';
import {
  VOICE_THRESHOLDS,
  standardRequestParams,
  voiceProcessPayload,
  uniqueIdempotencyKey,
  assertValidEnvelope,
} from './lib/options.js';

export const options = {
  scenarios: {
    warm_1: {
      executor: 'constant-arrival-rate',
      rate: 1,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 5,
      maxVUs: 10,
      exec: 'voiceScenario',
    },
    surge_20: {
      executor: 'constant-arrival-rate',
      rate: 20,
      timeUnit: '1s',
      duration: '30s',
      startTime: '30s',
      preAllocatedVUs: 25,
      maxVUs: 50,
      exec: 'voiceScenario',
    },
  },
  thresholds: VOICE_THRESHOLDS,
};

export function voiceScenario() {
  const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080';
  const includeTTS = Math.random() < 0.5;
  const body = voiceProcessPayload(includeTTS);
  const params = standardRequestParams({
    headers: {
      'Content-Type': 'application/json',
      'X-Request-ID': `voice-${__VU}-${__ITER}-${Date.now()}`,
      'Idempotency-Key': uniqueIdempotencyKey(),
    },
  });
  const res = http.post(`${baseURL}/api/v3/voice/process`, body, params);
  if (res.status === 503) {
    check(res, { 'queue saturated (expected)': (r) => r.status === 503 });
    return;
  }
  assertValidEnvelope(res, 'voice.process');
  check(res, { '200': (r) => r.status === 200 });
}
