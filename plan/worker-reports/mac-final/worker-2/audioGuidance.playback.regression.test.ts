// Focused regression check for Worker 2: audio playback ownership,
// cancellation, and replay safeguards. Not part of the shared test suite —
// isolated scratch verification only. Run manually against the scratch copy.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { AudioPlaybackGuard, type AudioMetadata } from './audioGuidance.ts';

function mockAudio() {
  let paused = false;
  let currentTime = 0;
  return {
    paused,
    currentTime,
    pause() {
      this.paused = true;
    },
    set currentTime_(v: number) {
      currentTime = v;
    },
  } as unknown as HTMLAudioElement & { paused: boolean; currentTime: number };
}

const meta: AudioMetadata = {
  audio_b64: 'AAAA',
  content_type: 'audio/wav',
  byte_size: 4,
  checksum_sha256: '0'.repeat(64),
  source_version: 1,
  template_version: 1,
  data_version: 'v1',
  language: 'hi-IN',
  settings: { sample_rate: 16000, channels: 1, bit_depth: 16 },
};

test('regression: already-playing audio stops when invalidated', () => {
  const guard = new AudioPlaybackGuard();
  const audio = mockAudio();
  guard.setActive(audio, guard.currentGeneration);
  assert.equal(guard.activePlayback, audio);
  assert.equal(audio.paused, false);

  guard.invalidate();

  assert.equal(guard.activePlayback, null);
  assert.equal(audio.paused, true, 'active audio must be paused on invalidation');
});

test('regression: pending autoplay cannot replay stale content after invalidation', () => {
  const guard = new AudioPlaybackGuard();
  const pendingAudio = mockAudio();
  guard.setPending({
    audio: pendingAudio,
    metadata: meta,
    expectedLanguage: 'hi-IN',
    expectedDataVersion: 'v1',
  });
  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), true);

  guard.invalidate(); // e.g. language change before user taps replay

  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), false);
  assert.equal(guard.consumePending(), null, 'stale pending audio must not be handed back for playback');
});

test('regression: new valid audio continues playing after an older generation is superseded', () => {
  const guard = new AudioPlaybackGuard();
  const oldAudio = mockAudio();
  guard.setActive(oldAudio, guard.currentGeneration);

  guard.invalidate(); // supersede (e.g. new voice request)
  const newGen = guard.currentGeneration;
  const newAudio = mockAudio();
  guard.setActive(newAudio, newGen);

  assert.equal(oldAudio.paused, true, 'superseded audio was stopped');
  assert.equal(guard.activePlayback, newAudio, 'new audio is retained as active');
  assert.equal(newAudio.paused, false, 'new audio was not touched by the prior invalidation');
});

test('regression: rapid supersession cannot revive old audio (late setActive for stale generation is stopped, not retained)', () => {
  const guard = new AudioPlaybackGuard();
  const gen0 = guard.currentGeneration;
  const staleAudio = mockAudio();

  // Simulate a slow async play() resolving after two rapid invalidations.
  guard.invalidate();
  guard.invalidate();

  // The stale callback (captured gen0) tries to register its audio as active.
  guard.setActive(staleAudio, gen0);

  assert.equal(staleAudio.paused, true, 'late-arriving stale audio must be stopped, never retained');
  assert.equal(guard.activePlayback, null, 'guard must not treat stale audio as active');
});

test("regression: verifyAndPlayAudio's own invalidate() call does not stop the clip it just started", () => {
  // Mirrors the interaction traced in main.ts: invalidate() is called right
  // after a successful play() to clear generation/pending, and the just-started
  // clip is registered as active AFTER that call, under the NEW generation.
  const guard = new AudioPlaybackGuard();
  const audio = mockAudio();

  guard.invalidate(); // simulates the call verifyAndPlayAudio makes post-play()
  guard.setActive(audio, guard.currentGeneration); // registered under the new/current generation

  assert.equal(audio.paused, false, 'the newly started clip must remain playing');
  assert.equal(guard.activePlayback, audio);
});

test('regression: finished/failed playback releases the active reference without pausing', () => {
  const guard = new AudioPlaybackGuard();
  const audio = mockAudio();
  guard.setActive(audio, guard.currentGeneration);

  guard.releaseActive(audio); // simulates 'ended' or 'error' event

  assert.equal(guard.activePlayback, null);
  assert.equal(audio.paused, false, 'natural completion should not be reported as a forced pause');
});

test('regression: releaseActive is a no-op for an element that is not currently active', () => {
  const guard = new AudioPlaybackGuard();
  const a = mockAudio();
  const b = mockAudio();
  guard.setActive(a, guard.currentGeneration);

  guard.releaseActive(b); // unrelated element

  assert.equal(guard.activePlayback, a, 'unrelated release must not clear the real active reference');
});

test('regression: ordinary autoplay rejection still queues pending, and user-gesture consumePending is honored once', () => {
  const guard = new AudioPlaybackGuard();
  const audio = mockAudio();
  guard.setPending({
    audio,
    metadata: meta,
    expectedLanguage: 'hi-IN',
    expectedDataVersion: 'v1',
  });

  assert.equal(guard.canPlayPending('CURRENT', 'v1', 'hi-IN'), true, 'valid pending autoplay survives until consumed');
  const consumed = guard.consumePending();
  assert.equal(consumed, audio);
  assert.equal(guard.consumePending(), null, 'pending cannot be consumed twice');
});
