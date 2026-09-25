import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  buildAudioMetadata,
  isAudioValidForReplay,
  processVoiceEnvelope,
  type AudioMetadata,
  type VoiceResponseEnvelope,
} from './audioGuidance.ts';

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
    // Action proposal is preserved and returned so map can execute!
    assert.ok(outcome.proposal);
    // But caption text was expected and is missing -> reports honest unavailable caption
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
        audio_b64: 'UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=',
        content_type: 'audio/wav',
        byte_size: 44,
        source_version: 1,
        template_version: 2,
        language: 'hi-IN',
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

test('buildAudioMetadata: rejects missing or non-positive provenance', () => {
  const validAudio = {
    audio_b64: 'dGVzdA==',
    source_version: 1,
    template_version: 1,
    language: 'hi-IN',
  };

  // Missing data_version
  assert.equal(buildAudioMetadata(validAudio, {}, undefined), undefined);
  assert.equal(buildAudioMetadata(validAudio, {}, ''), undefined);

  // Non-positive or non-integer source_version
  assert.equal(buildAudioMetadata({ ...validAudio, source_version: 0 }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, source_version: -1 }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, source_version: undefined }, {}, 'v1'), undefined);

  // Non-positive or non-integer template_version
  assert.equal(buildAudioMetadata({ ...validAudio, template_version: 0 }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, template_version: -2 }, {}, 'v1'), undefined);

  // Empty or invalid language
  assert.equal(buildAudioMetadata({ ...validAudio, language: '' }, {}, 'v1'), undefined);
  assert.equal(buildAudioMetadata({ ...validAudio, language: '   ' }, {}, 'v1'), undefined);

  // Template version mismatch
  assert.equal(
    buildAudioMetadata(validAudio, { template_version: 2 }, 'v1'),
    undefined
  );

  // Valid
  const built = buildAudioMetadata(validAudio, { template_version: 1 }, 'v1');
  assert.ok(built);
  assert.equal(built.source_version, 1);
  assert.equal(built.template_version, 1);
  assert.equal(built.data_version, 'v1');
  assert.equal(built.language, 'hi-IN');
});

test('isAudioValidForReplay: verifies freshness, version matching, and positive provenance', () => {
  const meta: AudioMetadata = {
    audio_b64: 'dGVzdA==',
    source_version: 1,
    template_version: 2,
    data_version: 'v1',
    language: 'hi-IN',
  };

  // Matching context -> valid
  assert.equal(isAudioValidForReplay(meta, 'CURRENT', 'v1', 'hi-IN'), true);

  // Stale guidance
  assert.equal(isAudioValidForReplay(meta, 'STALE', 'v1', 'hi-IN'), false);
  assert.equal(isAudioValidForReplay(meta, 'UNAVAILABLE', 'v1', 'hi-IN'), false);

  // Language mismatch (e.g. citizen switched UI to Malayalam)
  assert.equal(isAudioValidForReplay(meta, 'CURRENT', 'v1', 'ml-IN'), false);

  // Data version mismatch (e.g. new data version deployed)
  assert.equal(isAudioValidForReplay(meta, 'CURRENT', 'v2', 'hi-IN'), false);

  // Non-positive provenance
  assert.equal(isAudioValidForReplay({ ...meta, source_version: 0 }, 'CURRENT', 'v1', 'hi-IN'), false);
  assert.equal(isAudioValidForReplay({ ...meta, template_version: -1 }, 'CURRENT', 'v1', 'hi-IN'), false);
});
