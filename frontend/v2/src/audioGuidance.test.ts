import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  buildAudioMetadata,
  isAudioValidForReplay,
  verifyAudioIntegrity,
  AudioPlaybackGuard,
  processVoiceEnvelope,
  buildVoicePipelineRequest,
  evaluateReadinessState,
  shouldDropRecordedAudio,
  pickSupportedRecorderMimeType,
  type AudioMetadata,
  type VoiceResponseEnvelope,
} from './audioGuidance.ts';

// Deterministic minimal 44-byte WAV header fixture
const sampleWavB64 = 'UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=';
const sampleWavBytes = Buffer.from(sampleWavB64, 'base64');
const sampleWavSha256 = '5b517b506f635e251ee0d7020062f1ce375cc3a92f1f445e71be6995f4a591f1';

test('processVoiceEnvelope: allows silent camera actions (ZOOM) without speech or error', () => {
  const envelope: VoiceResponseEnvelope = {
    data_version: 'EXERCISE-V1',
    data: {
      data_version: 'EXERCISE-V1',
      state: 'OK',
      validated_proposal: {
        schema_version: '3.0',
        status: 'OK',
        intent: 'ZOOM',
        actions: [{ type: 'ZOOM', direction: 'IN', steps: 1 }],
        speech_key: null,
      },
    },
  };

  const outcome = processVoiceEnvelope(envelope);
  assert.equal(outcome.kind, 'OK');
  if (outcome.kind === 'OK') {
    assert.equal(outcome.captionUnavailable, false);
    assert.equal(outcome.guidanceSpeechExpected, false);
    assert.equal(outcome.template_text, undefined);
    assert.equal(outcome.audio, undefined);
    assert.ok(outcome.proposal);
  }
});

test('processVoiceEnvelope: allows ARRIVAL_CONFIRMATION panel without speech or error', () => {
  const envelope: VoiceResponseEnvelope = {
    data_version: 'EXERCISE-V1',
    data: {
      data_version: 'EXERCISE-V1',
      state: 'OK',
      validated_proposal: {
        schema_version: '3.0',
        status: 'OK',
        intent: 'ARRIVE',
        actions: [{ type: 'OPEN_PANEL', panel: 'ARRIVAL_CONFIRMATION' }],
        speech_key: null,
      },
    },
  };

  const outcome = processVoiceEnvelope(envelope);
  assert.equal(outcome.kind, 'OK');
  if (outcome.kind === 'OK') {
    assert.equal(outcome.captionUnavailable, false);
    assert.equal(outcome.guidanceSpeechExpected, false);
    assert.equal(outcome.template_text, undefined);
  }
});

test('processVoiceEnvelope: guidance intent with speech key returns captionUnavailable when text missing', () => {
  const envelope: VoiceResponseEnvelope = {
    data_version: 'EXERCISE-V1',
    data: {
      data_version: 'EXERCISE-V1',
      state: 'OK',
      validated_proposal: {
        schema_version: '3.0',
        status: 'OK',
        intent: 'DESTINATION',
        actions: [{ type: 'SHOW_CHOICES', target_ids: ['FACDEMO-1'] }],
        speech_key: 'DESTINATION_CHOICES',
      },
      template: {
        speech_key: 'DESTINATION_CHOICES',
      },
    },
  };

  const outcome = processVoiceEnvelope(envelope);
  assert.equal(outcome.kind, 'OK');
  if (outcome.kind === 'OK') {
    assert.ok(outcome.proposal);
    assert.equal(outcome.captionUnavailable, true);
    assert.equal(outcome.guidanceSpeechExpected, true);
    assert.equal(outcome.template_text, undefined);
  }
});

test('processVoiceEnvelope: destination with valid approved template and audio returns complete payload', () => {
  const envelope: VoiceResponseEnvelope = {
    data_version: 'EXERCISE-V1',
    data: {
      data_version: 'EXERCISE-V1',
      state: 'OK',
      validated_proposal: {
        schema_version: '3.0',
        status: 'OK',
        intent: 'DESTINATION',
        actions: [{ type: 'SHOW_CHOICES', target_ids: ['FACDEMO-1'] }],
        speech_key: 'DESTINATION_CHOICES',
      },
      template: {
        speech_key: 'DESTINATION_CHOICES',
        text: 'यहाँ सुरक्षित आश्रय स्थल के विकल्प हैं।',
        template_version: 2,
      },
      audio: {
        audio_b64: sampleWavB64,
        content_type: 'audio/wav',
        byte_size: 44,
        checksum_sha256: sampleWavSha256,
        source_version: 1,
        template_version: 2,
        language: 'hi-IN',
        settings: {
          sample_rate: 16000,
          channels: 1,
          bit_depth: 16,
        },
      },
    },
  };

  const outcome = processVoiceEnvelope(envelope);
  assert.equal(outcome.kind, 'OK');
  if (outcome.kind === 'OK') {
    assert.equal(outcome.captionUnavailable, false);
    assert.equal(outcome.guidanceSpeechExpected, true);
    assert.equal(outcome.template_text, 'यहाँ सुरक्षित आश्रय स्थल के विकल्प हैं।');
    assert.ok(outcome.audio);
    assert.equal(outcome.audio.data_version, 'EXERCISE-V1');
    assert.equal(outcome.audio.source_version, 1);
    assert.equal(outcome.audio.template_version, 2);
    assert.equal(outcome.audio.language, 'hi-IN');
  }
});

test('processVoiceEnvelope: destination with invalid/missing audio metadata preserves map actions', () => {
  const envelope: VoiceResponseEnvelope = {
    data_version: 'EXERCISE-V1',
    data: {
      data_version: 'EXERCISE-V1',
      state: 'OK',
      validated_proposal: {
        schema_version: '3.0',
        status: 'OK',
        intent: 'DESTINATION',
        actions: [{ type: 'SHOW_CHOICES', target_ids: ['FACDEMO-1'] }],
        speech_key: 'DESTINATION_CHOICES',
      },
      template: {
        speech_key: 'DESTINATION_CHOICES',
        text: 'यहाँ सुरक्षित आश्रय स्थल के विकल्प हैं।',
        template_version: 1,
      },
      audio: {
        audio_b64: sampleWavB64,
        // Missing content_type, checksum, byte_size, settings!
      },
    },
  };

  const outcome = processVoiceEnvelope(envelope);
  assert.equal(outcome.kind, 'OK');
  if (outcome.kind === 'OK') {
    // Valid map proposal is 100% preserved
    assert.ok(outcome.proposal);
    assert.equal(outcome.template_text, 'यहाँ सुरक्षित आश्रय स्थल के विकल्प हैं।');
    // Audio is blocked (undefined)
    assert.equal(outcome.audio, undefined);
  }
});

test('processVoiceEnvelope: handles CLARIFY without executing map actions', () => {
  const envelope: VoiceResponseEnvelope = {
    data_version: 'EXERCISE-V1',
    data: {
      state: 'CLARIFY',
      validated_proposal: {
        schema_version: '3.0',
        status: 'CLARIFY',
        clarification_ids: ['SZDEMO-1', 'FACDEMO-1'],
        actions: [],
      },
      template: {
        text: 'कृपया स्थान स्पष्ट करें।',
      },
    },
  };

  const outcome = processVoiceEnvelope(envelope);
  assert.equal(outcome.kind, 'CLARIFY');
  if (outcome.kind === 'CLARIFY') {
    assert.deepEqual(outcome.clarification_ids, ['SZDEMO-1', 'FACDEMO-1']);
    assert.equal(outcome.template_text, 'कृपया स्थान स्पष्ट करें।');
  }
});

test('processVoiceEnvelope: non-OK states (UNSUPPORTED, DATA_UNAVAILABLE, ERROR) return ERROR', () => {
  assert.equal(processVoiceEnvelope({ data: { state: 'UNSUPPORTED' } }).kind, 'ERROR');
  assert.equal(processVoiceEnvelope({ data: { state: 'DATA_UNAVAILABLE' } }).kind, 'ERROR');
  assert.equal(processVoiceEnvelope({ data: { state: 'ERROR' } }).kind, 'ERROR');
  assert.equal(processVoiceEnvelope({ data: { state: 'MODEL_UNAVAILABLE' } }).kind, 'ERROR');
  assert.equal(processVoiceEnvelope(null).kind, 'ERROR');
});

test('buildAudioMetadata: rejects missing checksum, size, content type, and invalid settings', () => {
  const validAudio = {
    audio_b64: sampleWavB64,
    content_type: 'audio/wav',
    byte_size: 44,
    checksum_sha256: sampleWavSha256,
    source_version: 1,
    template_version: 1,
    language: 'hi-IN',
    settings: {
      sample_rate: 16000,
      channels: 1,
      bit_depth: 16,
    },
  };

  // Valid matching audio passes
  const passed = buildAudioMetadata(validAudio, { template_version: 1 }, 'v1');
  assert.ok(passed);
  assert.equal(passed.byte_size, 44);
  assert.equal(passed.content_type, 'audio/wav');
  assert.equal(passed.checksum_sha256, sampleWavSha256);

  // Missing content_type
  assert.equal(buildAudioMetadata({ ...validAudio, content_type: undefined }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, content_type: 'video/mp4' }, {}, 'v1'), undefined);

  // Missing or invalid byte_size
  assert.equal(buildAudioMetadata({ ...validAudio, byte_size: undefined }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, byte_size: 0 }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, byte_size: -1 }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, byte_size: 800 * 1024 }, {}, 'v1'), undefined); // exceeds ceiling

  // Missing or invalid checksum_sha256
  assert.equal(buildAudioMetadata({ ...validAudio, checksum_sha256: undefined }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, checksum_sha256: 'short' }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, checksum_sha256: 'xyz'.repeat(22) }, {}, 'v1'), undefined); // non-hex

  // Missing or invalid settings
  assert.equal(buildAudioMetadata({ ...validAudio, settings: undefined }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, settings: { sample_rate: 0, channels: 1, bit_depth: 16 } }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, settings: { sample_rate: 16000, channels: 0, bit_depth: 16 } }, {}, 'v1'), undefined);

  // Missing data_version
  assert.equal(buildAudioMetadata(validAudio, {}, undefined), undefined);
  assert.equal(buildAudioMetadata(validAudio, {}, ''), undefined);

  // Non-positive source_version
  assert.equal(buildAudioMetadata({ ...validAudio, source_version: 0 }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, source_version: -1 }, {}, 'v1'), undefined);

  // Non-positive template_version
  assert.equal(buildAudioMetadata({ ...validAudio, template_version: 0 }, {}, 'v1'), undefined);

  // Template version mismatch
  assert.equal(buildAudioMetadata(validAudio, { template_version: 2 }, 'v1'), undefined);
});

test('verifyAudioIntegrity: cryptographically verifies exact byte count and SHA-256 digest', async () => {
  const meta: AudioMetadata = {
    audio_b64: sampleWavB64,
    content_type: 'audio/wav',
    byte_size: 44,
    checksum_sha256: sampleWavSha256,
    source_version: 1,
    template_version: 1,
    data_version: 'v1',
    language: 'hi-IN',
    settings: { sample_rate: 16000, channels: 1, bit_depth: 16 },
  };

  // Valid matching audio passes verification with subtle crypto
  const passRes = await verifyAudioIntegrity(meta, crypto.subtle);
  assert.equal(passRes.success, true);

  // Byte size mismatch fails
  const sizeMismatchRes = await verifyAudioIntegrity({ ...meta, byte_size: 45 }, crypto.subtle);
  assert.equal(sizeMismatchRes.success, false);
  assert.match(sizeMismatchRes.error || '', /byte size mismatch/);

  // Tampered checksum fails
  const tamperedChecksumRes = await verifyAudioIntegrity(
    { ...meta, checksum_sha256: '0'.repeat(64) },
    crypto.subtle
  );
  assert.equal(tamperedChecksumRes.success, false);
  assert.match(tamperedChecksumRes.error || '', /checksum mismatch/);

  // Missing crypto provider fails closed
  const noCryptoRes = await verifyAudioIntegrity(meta, null as any);
  assert.equal(noCryptoRes.success, false);
  assert.match(noCryptoRes.error || '', /Web Crypto API unavailable/);
});

test('isAudioValidForReplay: verifies freshness, active language, and positive provenance', () => {
  const meta: AudioMetadata = {
    audio_b64: sampleWavB64,
    content_type: 'audio/wav',
    byte_size: 44,
    checksum_sha256: sampleWavSha256,
    source_version: 1,
    template_version: 2,
    data_version: 'v1',
    language: 'hi-IN',
    settings: { sample_rate: 16000, channels: 1, bit_depth: 16 },
  };

  // Matching context -> valid
  assert.equal(isAudioValidForReplay(meta, 'CURRENT', 'v1', 'hi-IN'), true);

  // Stale guidance
  assert.equal(isAudioValidForReplay(meta, 'STALE', 'v1', 'hi-IN'), false);
  assert.equal(isAudioValidForReplay(meta, 'UNAVAILABLE', 'v1', 'hi-IN'), false);

  // Language mismatch
  assert.equal(isAudioValidForReplay(meta, 'CURRENT', 'v1', 'ml-IN'), false);

  // Data version mismatch
  assert.equal(isAudioValidForReplay(meta, 'CURRENT', 'v2', 'hi-IN'), false);

  // Corrupted or missing integrity fields
  assert.equal(isAudioValidForReplay({ ...meta, byte_size: 0 }, 'CURRENT', 'v1', 'hi-IN'), false);
  assert.equal(isAudioValidForReplay({ ...meta, checksum_sha256: '' }, 'CURRENT', 'v1', 'hi-IN'), false);
});

test('AudioPlaybackGuard: language, version, or freshness change during async verification cancels playback', () => {
  const guard = new AudioPlaybackGuard();
  const startGen = guard.currentGeneration;

  const meta: AudioMetadata = {
    audio_b64: sampleWavB64,
    content_type: 'audio/wav',
    byte_size: 44,
    checksum_sha256: sampleWavSha256,
    source_version: 1,
    template_version: 1,
    data_version: 'v1',
    language: 'hi-IN',
    settings: { sample_rate: 16000, channels: 1, bit_depth: 16 },
  };

  const dummyAudio: any = { play: async () => {} };
  guard.setPending({
    audio: dummyAudio,
    metadata: meta,
    expectedLanguage: 'hi-IN',
    expectedDataVersion: 'v1',
  });

  // Valid before invalidation
  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), true);

  // Invalidation (e.g. language switched to ml-IN)
  guard.invalidate();
  assert.notEqual(guard.currentGeneration, startGen);
  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), false);
  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'ml-IN'), false);
});

test('AudioPlaybackGuard: blocked autoplay followed by invalidation cannot be replayed', () => {
  const guard = new AudioPlaybackGuard();
  const meta: AudioMetadata = {
    audio_b64: sampleWavB64,
    content_type: 'audio/wav',
    byte_size: 44,
    checksum_sha256: sampleWavSha256,
    source_version: 1,
    template_version: 1,
    data_version: 'v1',
    language: 'hi-IN',
    settings: { sample_rate: 16000, channels: 1, bit_depth: 16 },
  };

  const mockAudio: any = { play: () => {} };
  guard.setPending({
    audio: mockAudio,
    metadata: meta,
    expectedLanguage: 'hi-IN',
    expectedDataVersion: 'v1',
  });

  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), true);

  // Guidance becomes STALE or revoked
  assert.equal(guard.canPlayPending('STALE', 'v1', 'hi-IN'), false);

  // Language changes
  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'en-IN'), false);

  // Data version changes
  assert.equal(guard.canPlayPending('CURRENT', 'v2', 'hi-IN'), false);

  // Guard invalidation clears pending audio completely
  guard.invalidate();
  assert.equal(guard.pendingAutoplay, null);
  assert.equal(guard.consumePending(), null);
});

test('buildVoicePipelineRequest: strictly builds PipelineRequest envelope for audio and transcript', () => {
  const audioReq = buildVoicePipelineRequest(
    { kind: 'audio', body_b64: sampleWavB64, content_type: 'audio/wav' },
    'ml-IN',
    'DEMO-EXERCISE',
    'req-custom-1'
  );
  assert.equal(audioReq.request_id, 'req-custom-1');
  assert.equal(audioReq.jurisdiction, 'DEMO-EXERCISE');
  assert.equal(audioReq.language, 'ml-IN');
  assert.equal(audioReq.input.kind, 'audio');
  if (audioReq.input.kind === 'audio') {
    assert.equal(audioReq.input.body_b64, sampleWavB64);
    assert.equal(audioReq.input.content_type, 'audio/wav');
  }
  assert.deepEqual(audioReq.render, { kind: 'tts' });

  const transcriptReq = buildVoicePipelineRequest(
    { kind: 'transcript', text: 'show my route' },
    'hi-IN'
  );
  assert.equal(transcriptReq.jurisdiction, 'DEMO-EXERCISE');
  assert.equal(transcriptReq.language, 'hi-IN');
  assert.equal(transcriptReq.input.kind, 'transcript');
  if (transcriptReq.input.kind === 'transcript') {
    assert.equal(transcriptReq.input.text, 'show my route');
    assert.equal(transcriptReq.input.confidence, 1.0);
  }
  assert.deepEqual(transcriptReq.render, { kind: 'tts' });
});

test('evaluateReadinessState: fail-closed on disconnected/degraded backend and never returns demo on failure', () => {
  // Connection error (fetch throws or fails)
  const failConn = evaluateReadinessState(null, null);
  assert.equal(failConn.runtime, 'blocked');
  assert.equal(failConn.runtimeDetail, 'disconnected');

  // 503 Service Unavailable
  const fail503 = evaluateReadinessState(503, { status: 'NOT_READY' });
  assert.equal(fail503.runtime, 'blocked');
  assert.equal(fail503.runtimeDetail, 'blocked');

  // 200 with non-READY status
  const nonReady = evaluateReadinessState(200, { status: 'DEGRADED' });
  assert.equal(nonReady.runtime, 'blocked');
  assert.equal(nonReady.runtimeDetail, 'blocked');

  // 200 with missing body
  const emptyBody = evaluateReadinessState(200, null);
  assert.equal(emptyBody.runtime, 'blocked');
  assert.equal(emptyBody.runtimeDetail, 'disconnected');

  // 200 with READY
  const ready = evaluateReadinessState(200, { status: 'READY' });
  assert.equal(ready.runtime, 'demo');
  assert.equal(ready.runtimeDetail, 'responding');
});

test('shouldDropRecordedAudio: enforces cancellation and foreground execution guards', () => {
  // Cancelled recording must be dropped
  assert.equal(shouldDropRecordedAudio(true, false), true);

  // Hidden/background document must be dropped
  assert.equal(shouldDropRecordedAudio(false, true), true);

  // Both cancelled and hidden must be dropped
  assert.equal(shouldDropRecordedAudio(true, true), true);

  // Active foreground recording is kept
  assert.equal(shouldDropRecordedAudio(false, false), false);
});


test('pickSupportedRecorderMimeType: only returns types the backend transcription allow-list accepts', () => {
  const only = (...types: string[]) => (mime: string) => types.includes(mime);
  assert.equal(pickSupportedRecorderMimeType(only('audio/webm;codecs=opus', 'audio/webm')), 'audio/webm;codecs=opus');
  assert.equal(pickSupportedRecorderMimeType(only('audio/ogg;codecs=opus')), 'audio/ogg;codecs=opus');
  // Safari-style recorder: audio/mp4 is not on the backend allow-list, so the mic must not open.
  assert.equal(pickSupportedRecorderMimeType(only('audio/mp4', 'audio/aac')), null);
  assert.equal(pickSupportedRecorderMimeType(() => false), null);
});

function fakeAudio() {
  return { pauses: 0, pause() { this.pauses++; } } as unknown as HTMLAudioElement & { pauses: number };
}

test('AudioPlaybackGuard: invalidation stops the clip that is already playing', () => {
  const guard = new AudioPlaybackGuard();
  const playing = fakeAudio();
  guard.claimPlayback(playing);
  assert.equal(playing.pauses, 0, 'claiming must not stop the clip that just started');
  guard.invalidate(); // language change, hide, offline, revocation, new request
  assert.equal(playing.pauses, 1);
  guard.invalidate();
  assert.equal(playing.pauses, 1, 'a stopped clip is released, not paused again');
});

test('AudioPlaybackGuard: a newly started clip supersedes older playback and queued autoplay', () => {
  const guard = new AudioPlaybackGuard();
  const older = fakeAudio();
  const newer = fakeAudio();
  guard.claimPlayback(older);
  guard.setPending({ audio: fakeAudio(), metadata: {} as AudioMetadata, expectedLanguage: 'en-IN', expectedDataVersion: 'v1' });
  const genBefore = guard.currentGeneration;
  guard.claimPlayback(newer);
  assert.equal(older.pauses, 1);
  assert.equal(newer.pauses, 0);
  assert.equal(guard.pendingAutoplay, null);
  assert.ok(guard.currentGeneration > genBefore, 'in-flight verifications from before the new clip become stale');
});
