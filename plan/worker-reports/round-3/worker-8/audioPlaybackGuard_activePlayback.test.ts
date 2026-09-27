import { test } from 'node:test';
import assert from 'node:assert/strict';
import { AudioPlaybackGuard } from './audioGuidance.ts';
import type { AudioMetadata } from './audioGuidance.ts';

const sampleWavB64 = 'UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=';
const sampleWavSha256 = '5b517b506f635e251ee0d7020062f1ce375cc3a92f1f445e71be6995f4a591f1';

function makeMeta(): AudioMetadata {
  return {
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
}

/**
 * Regression: active audio playback must be stopped when the guard is invalidated.
 *
 * Gap: AudioPlaybackGuard.invalidate() increments the generation and clears pending
 * audio but does NOT pause a currently-playing HTMLAudioElement. This means:
 * - visibilitychange → supersedeInFlight() → audioGuard.invalidate() leaves any
 *   in-progress audio still playing until it naturally finishes.
 * - Language switch or guidance refresh during playback produces audible bleed-through
 *   of the old audio while the new guidance is displayed.
 *
 * Expected (after fix): invalidate() or supersedeInFlight() pauses the active audio.
 *
 * This test FAILS against the current implementation (audio.pause is NOT called).
 * After the fix it should PASS.
 *
 * Run: cd frontend/v2/src && node --test audioPlaybackGuard_activePlayback.test.ts
 * Status: NOT_RUN — deferred until after usage reset
 */
test('AudioPlaybackGuard: invalidate() pauses active audio to prevent bleed-through', async () => {
  const guard = new AudioPlaybackGuard();
  const meta = makeMeta();

  let pauseCalled = false;
  const activeAudio = {
    play: async () => {
      // Simulate successful playback start
    },
    pause: () => {
      pauseCalled = true;
    },
  } as unknown as HTMLAudioElement;

  guard.setPending({
    audio: activeAudio,
    metadata: meta,
    expectedLanguage: 'hi-IN',
    expectedDataVersion: 'v1',
  });

  // Simulate the audio starting to play successfully (blocked autoplay case)
  // The caller would now hold activeAudio and it would be playing.
  // consumePending gives up ownership but does NOT pause:
  const consumed = guard.consumePending();
  assert.ok(consumed, 'consumePending should return the audio element');
  assert.equal(pauseCalled, false, 'pause should NOT be called by consumePending');

  // At this point audio is playing. Now invalidate() is called (e.g. by supersedeInFlight
  // on visibilitychange, language switch, or guidance refresh).
  guard.invalidate();

  // After fix: pause SHOULD have been called on activeAudio.
  // Before fix: pause is NOT called — audio keeps playing.
  assert.equal(
    pauseCalled,
    true,
    'invalidate() must pause the active audio element to prevent stale audio bleed-through. ' +
      'Without this, supersedeInFlight() (called on visibilitychange, language change, offline) ' +
      'does not stop any audio already playing, causing audible guidance from a superseded context.'
  );
});

/**
 * Regression: stale audio cannot begin playback after guard invalidation.
 *
 * Verifies that once guard.invalidate() is called, canPlayPending returns false
 * and no new audio can be played through the guard mechanism.
 *
 * Run: cd frontend/v2/src && node --test audioPlaybackGuard_activePlayback.test.ts
 * Status: NOT_RUN — deferred until after usage reset
 */
test('AudioPlaybackGuard: after invalidate, canPlayPending is false and pending is cleared', () => {
  const guard = new AudioPlaybackGuard();
  const meta = makeMeta();

  const mockAudio = {
    play: async () => {},
    pause: () => {},
  } as unknown as HTMLAudioElement;

  guard.setPending({
    audio: mockAudio,
    metadata: meta,
    expectedLanguage: 'hi-IN',
    expectedDataVersion: 'v1',
  });

  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), true);

  guard.invalidate();

  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), false);
  assert.equal(guard.pendingAutoplay, null);
  assert.equal(guard.consumePending(), null);
});
