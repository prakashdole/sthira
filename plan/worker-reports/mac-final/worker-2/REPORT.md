# Worker 2 report — audio playback ownership, cancellation, replay safeguards

## Status: DONE (isolated scratch copy; no shared source/tests edited)

## Root cause traced

`AudioPlaybackGuard` (frontend/v2/src/audioGuidance.ts) tracked a generation
counter and a queued `pending` autoplay-blocked element, but never retained a
reference to the `HTMLAudioElement` that was actively playing. `invalidate()`
bumped the generation and cleared `pending` only. Every supersession path in
main.ts (language change via `SET_LANGUAGE`, `supersedeInFlight` on
hide/new-request, `offline` handler, `revokeRoute`, guidance-fetch error
paths) already calls `audioGuard.invalidate()`, but none of them had any
audio element to stop, so already-playing clips kept playing after being
superseded.

`verifyAndPlayAudio` also calls `audioGuard.invalidate()` immediately after
its own successful `.play()` (frontend/v2/src/main.ts, existing code). This
call bumps the generation *before* the just-started clip is registered, so
making `invalidate()` stop active audio does not stop the clip that just
started — it only stops whatever was active from a *previous* generation.
Traced and confirmed via the regression test
"verifyAndPlayAudio's own invalidate() call does not stop the clip it just
started".

## Fix (frontend/v2/src/audioGuidance.ts, frontend/v2/src/main.ts)

- `AudioPlaybackGuard` gained a private `active` slot + `activePlayback`
  getter, `setActive(audio, forGeneration)`, and `releaseActive(audio)`.
- `invalidate()` now also stops+releases the active element (pause +
  reset `currentTime`), in addition to its existing generation bump and
  `pending = null`.
- `setActive` checks the caller's captured generation against the guard's
  current generation: if superseded, it stops the incoming audio instead of
  retaining it (closes the "rapid supersession revives old audio" race).
- In `verifyAndPlayAudio`'s play-success branch, after the existing
  `audioGuard.invalidate()` call, the new clip is registered via
  `audioGuard.setActive(audio, audioGuard.currentGeneration)` (the
  post-invalidate/current generation, not the stale captured `currentGen`),
  and `ended`/`error` listeners call `releaseActive` to drop finished/failed
  references without falsely reporting them as force-stopped.
- Pending-autoplay queue behavior (clearing on invalidation, generation/
  freshness/language/version checks in `canPlayPending`) is unchanged — this
  was already correct and is distinct from active-playback stopping per the
  task's explicit warning.
- The existing "tap to play" recovery path (`data-action="listen"` /
  `data-action="audio-play-modal"` handlers) already re-runs
  `verifyAndPlayAudio` from scratch on user gesture rather than consuming the
  queued pending element; this re-validates freshness/language/version at
  the moment of the tap and was left untouched — it already satisfies
  "preserve valid user-gesture replay after autoplay denial" and was not
  part of the confirmed defect.

Diff is minimal and surgical: 1 doc comment + ~45 new lines in
`audioGuidance.ts` (new tracking fields/methods only, no existing method
signatures changed), 3 lines added in `main.ts` (registration + 2 listener
lines). No other call sites, no test files, no unrelated files touched.

## Verification (real commands, isolated /tmp/worker2-scratch/v2 copy)

Base commit: `e8836922d90a153e2d3e571efde2381b28aa1a43` (see `base.commit`).
Base file hashes before any edit: see `base.sha256`.

1. Baseline (pre-fix) existing suite: 15/15 `audioGuidance.test.ts` passed
   (see reasoning trace; also reproducible via `git show <base>:...`).
2. Post-fix full suite incl. journey/operator/i18n/mapActions/emergency/
   audioGuidance (97 tests): **97 pass, 0 fail** — `test-results.post-fix.log`.
3. `npx tsc --noEmit`: clean, exit 0 — `typecheck.log`.
4. New focused regression file
   `audioGuidance.playback.regression.test.ts` (8 tests, included in this
   folder), run standalone: 8/8 pass post-fix.
5. Adversarial check: copied the *unmodified baseline* `audioGuidance.ts`
   back into the scratch copy and re-ran the same regression file —
   **6 of 8 failed** (`guard.setActive is not a function` / behavior
   absent), proving the tests exercise the real defect and are not
   tautological. Fixed file was restored immediately after, confirmed via
   `diff -q`.

Regression coverage maps directly to the requested cases:
- Already-playing audio stops when invalidated — direct `pause()`/
  `activePlayback` assertions.
- Pending autoplay cannot replay stale content — `canPlayPending` +
  `consumePending()` after `invalidate()`.
- New valid audio continues playing — old-generation audio paused, new one
  left running untouched.
- Rapid supersession cannot revive old audio — double `invalidate()` then a
  late `setActive` for the stale captured generation is stopped, not stored.
- Ordinary autoplay rejection / user-gesture recovery — `setPending` +
  `canPlayPending` + single-use `consumePending()`.

## Limitations / not evidence

- All checks run in Node's test runner against the pure `AudioPlaybackGuard`
  logic and `HTMLAudioElement`-shaped mocks (no `play()`/`pause()` calls
  return real promises or touch real media). No browser, no real device, no
  actual audio decoding was exercised — this is unit-level logic
  verification only, not a device/browser integration test.
- Per instruction, no allow-autoplay browser flag was used or treated as
  evidence for the denied-autoplay path; the denied-autoplay case is proven
  purely through the guard's `setPending`/`canPlayPending`/`consumePending`
  contract, not a real `NotAllowedError` from a browser engine.
- Did not attempt to launch a real browser (e.g. Playwright) — not installed
  in this environment and out of scope per "no installs" instruction.
- Recording/capture behavior (Worker 1's `mapActions.ts` and any recording
  cancellation logic) was not inspected beyond confirming it is untouched;
  the stray `mapActions.ts.bak/.orig/.rej/.copy` files already present in
  the shared tree belong to that work and were left exactly as found.

## Deliverables in this folder

- `candidate.patch` — unified diff, `audioGuidance.ts` + `main.ts`, against
  base commit `e8836922d90a153e2d3e571efde2381b28aa1a43`.
- `base.commit`, `base.sha256` — pre-edit provenance.
- `audioGuidance.playback.regression.test.ts` — the 8 focused checks (for
  reviewer reference; not wired into the shared `npm test` script).
- `typecheck.log`, `test-results.post-fix.log` — real command output.

No git mutations, commits, installs, or shared-file edits were performed.
All edits were made and verified only inside `/tmp/worker2-scratch/v2`.
