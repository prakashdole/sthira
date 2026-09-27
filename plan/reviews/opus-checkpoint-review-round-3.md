# Opus checkpoint review for numbered workers

Observed HEAD: `d95df9e609454877a91b7c82146b3c6368181b17` on CLEAN. This is a selective source/diff review, not final acceptance. No tests/builds/browser sessions were run; execution remains deferred by the user.

## Observed state

- `plan/worker-reports/opus.md` remains an AWAITING WORKER REPORT template. No newer committed final handoff was observed.
- `main.ts` already removes the transcript-to-place fallback on HTTP 503/504. This is an uncommitted six-line deletion. Worker 2 must prepare verification, not duplicate the fix.
- `stay_http_integration_test.go` already adds `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives`. It covers a foreign denial followed by owner success and final inventory. It has not been executed in this review.
- Audio, eval, CSS and other integration deltas remain uncommitted. Existing reports cannot establish acceptance of these changed contents.

## Additions to the work queue

1. Workers 17/18: distinguish final capacity after owner success from unchanged business state immediately after the foreign denial; reuse existing assertions before adding more. Review ignored JSON errors/nonempty stay ID without treating diagnostic weakness as an established product defect.
2. Worker 8: the old hidden-page replay finding is contradicted by supersedeInFlight clearing lastApprovedAudio. Investigate active playback separately.
3. Worker 21: review large speculative BenchmarkCapabilitySpec additions; preserve honest stub diagnostics and keep benchmark implementation out of scope.
4. Worker 25: a coverage-map test that logs other tests is documentation, not an executable acceptance test. Do not rename testing code into a production Go source file.
5. Workers 29/30: require exact final revision, dirty-state record, explicit deferred checks and Opus integration manifest before declaring completion.

## Source snapshot hashes

- `frontend/v2/src/main.ts`: `00dae4e67a63641bc62625c6eda95fd1d97846f9ece3202f4ac3c8e2a1291c09`
- `backend/internal/httpserver/stay_http_integration_test.go`: `053ed7b9efa56f35bd3f4b7a2f6fe6651e02839de2af1de3ab98ae0f84c946ad`
- `backend/internal/middleworker/eval/main.go`: `c41dec7aa68c3bfca8676962ff29ae9ff28cdca1674d8f0052867febe3c4971d`

Only numbered prompt files and this review note were edited for this review. Product code and existing worker outputs were preserved. Opus retains commit ownership.
