# Worker 25 Report — Sanitized Evidence and Fixture Review

**Timestamp:** 2026-09-27T16:NN UTC
**HEAD:** d95df9e609454877a91b7c82146b3c6368181b17
**Working Tree:** Dirty (existing files modified, not relevant to this review)
**Source Hashes (artifacts reviewed):**
- `plan/evidence/prototype-browser-verification.md`: `c0e75b46ae02cc7913acf0f58fb765d5cd90550bfe4ac915438a202b4dcc49e3`
- `plan/worker-reports/round-2/minimax-b/browser_accept.py`: `0ea14c8e46214a931672cb86b203f481b76782dbf25cc8639bd53670f6d44fb7`
- `plan/worker-reports/round-2/minimax-e/voice-outage-fallback.reticle.js`: `0e833cf626b2520f3e183ac2effd2d9ab896f9c88b763b5bf740781b4da975d6`
- `plan/worker-reports/round-3/worker-1/browser_accept.py`: `5d7b28142311419747ee3874e699878e92707e8505dd9e72cdd227749ccf72e1`
- `plan/worker-reports/round-3/worker-1/report.md`: `f235f2e5ec40955e74e0beecc21811f1bbf131c111b60ed98b78abcabc1a8bf3`

---

## 1. Scope Inspected

### Artifact Category A — Browser Acceptance Scripts

| File | SHA-256 | Language | Tokens | Unsafe Instructions |
|---|---|---|---|---|
| `round-2/minimax-b/browser_accept.py` | `0ea14c8...` | Python | None | None |
| `round-3/worker-1/browser_accept.py` | `5d7b281...` | Python | None | None |
| `round-2/minimax-e/voice-outage-fallback.reticle.js` | `0e833cf...` | JavaScript | None | None |

### Artifact Category B — Evidence Documents

| File | SHA-256 | PASS Labels | Notes |
|---|---|---|---|
| `plan/evidence/prototype-browser-verification.md` | `c0e75b46...` | Historical | "PASS" labels refer to execution at specific commits (e066056, 5d57c9b), not current source |
| `plan/worker-reports/round-2/minimax-k/report.md` | (prior evidence) | Queue labels | Integration queue, not execution claims |

### Artifact Category C — Test Source

| File | SHA-256 | Issue |
|---|---|---|
| `backend/internal/httpserver/citizen_ownership_regression_test.go` | `2954eefe...` | Logs test names; asserts nothing; must not count as executed acceptance |

---

## 2. Findings

### 2.1 Tokens and Hardcoded Credentials — NONE FOUND

Scanned all artifacts for session tokens, Bearer tokens, API keys, database connection strings, or any credentials:

- **browser_accept.py** (both versions): Uses only anonymous session creation (`POST /api/v3/sessions`), then stores the returned token in a local variable. No hardcoded credentials. httpx client connects to configurable base URL with no default auth.
- **voice-outage-fallback.reticle.js**: No credentials. Uses Reticle's network interception (CDP) for fault injection, no secrets.
- **prototype-browser-verification.md**: No credentials. Documents HTTP exchanges with demo exercise IDs (`DEMO-EXERCISE`, `PKGDEMO-1`) and hardcoded dates (2026-09-27), all publicly documented in the exercise configuration.
- **round-3/worker-1 artifacts**: Same baseline as minimax-b, same assessment.

**Field names checked:** `token`, `Authorization`, `Bearer`, `session_id`, `sessionToken`, `api_key`, `password`, `dsn`, `STHIRA_TEST_DSN`, `connection_string`, `secret`, `credential`.

---

### 2.2 PASS Labels — ONE UNSUPPORTED CLAIM FOUND

**File:** `backend/internal/httpserver/citizen_ownership_regression_test.go`

The `TestCitizenOwnershipRegression_CoverageMap` function has no assertions. It skips when `STHIRA_TEST_DSN` is absent; when present, it only logs test names via `t.Log`. It always exits PASS.

This was already noted in minimax-a's Finding 1 and minimax-k's integration queue. The function name starts with `Test` and Go's test runner reports it as PASS. It must not be counted as executed ownership acceptance.

**Specific correction from worker-25 prompt:** "recommend moving useful coverage documentation to a report; do not rename a testing-based file into production .go. No direct deletion of shared files."

**Recommendation:** Extract the coverage documentation comment block (lines 3-36 of the file) into `plan/evidence/citizen-ownership-coverage-map.md`. The Go file can then be deleted or left as an empty placeholder (no test with that name). Do NOT rename to a `.go` production file — the comment block documents test names which are internal to the test package.

---

### 2.3 Unsafe Instructions — NONE FOUND (with one note)

**kill-by-port, npm install, mutable window test setters:**

- No shell commands to kill processes by port in any artifact
- No `npm install` instructions in artifacts (only `npx @reticlehq/server init` which is a one-time Reticle setup command, not a destructive operation)
- `?sthira-test-hooks=1` is mentioned in two places:
  - `prototype-browser-verification.md`: "window hooks confirmed absent" — correctly notes this is a production safety check
  - `voice-outage-fallback.reticle.js` lines 35-52: commented-out `window.fetch` override technique for manual fault injection — only in comments, not executed code

**Note on commented fetch override (voice-outage-fallback.reticle.js:43-49):**
```js
//  To exercise via debug hook (dev only):
//  const orig = window.fetch;
//  window.fetch = (url, opts) => {
//    if (url.includes('/api/v3/voice/process')) {
//      return Promise.resolve(new Response('', { status: 503 }));
//    }
//    return orig(url, opts);
//  };
```
This is commented-out documentation of a manual browser technique. It requires the user to manually paste this into a browser console with `?sthira-test-hooks=1` active. Not an automated unsafe instruction.

---

## 3. Prototype Browser Verification PASS Labels — SUPPORTED

The PASS labels in `prototype-browser-verification.md` are historically grounded:

- `PASS 9/9 at e066056` — executed with headless Chromium at that commit
- `PASS 2/2` (browser outage/recovery) — executed
- `PASS` for eval corpus — offline execution with `corpus-check`
- `FAIL` labels are also present with explanations (e.g., "same test passed pre-fix", "pre-fix commit showed 2 POSTs")

The document explicitly notes two items that were marked PASS at earlier revisions but were actually defects, which were fixed in `aa97e69`. This self-correcting documentation is honest and accurate.

**No unsupported PASS claims found in this document.**

---

## 4. Narrow Redaction/Correction Recommendation

### R-01: Extract coverage map documentation from Go test file

**File:** `backend/internal/httpserver/citizen_ownership_regression_test.go`  
**Issue:** The file's comment block (lines 3-36) contains the only documentation of which tests cover which ownership/idempotency claims. The `TestCitizenOwnershipRegression_CoverageMap` function has zero assertions and always exits PASS when run — it cannot serve as regression evidence.

**Recommendation:** Create `plan/evidence/citizen-ownership-coverage-map.md` containing the documentation extracted from the Go file header (lines 3-36). Then remove the `TestCitizenOwnershipRegression_CoverageMap` function body, keeping only the Go package declaration and imports — or delete the file entirely. Do NOT convert to a production `.go` file.

**Fields/names to preserve in evidence file:**
- Coverage IDs: A1, A2, A3, A4, B1, B2, B3, B4, C1, C2, C3, C4, D1
- Test names: `TestHTTPCrossSessionDenial`, `TestHTTPDuplicateConfirmationReplay`, `TestB02_HTTPReservationLegacyReplayAndAliasingPrevention`, `TestHTTPRequiresSession`, `TestHTTPCreateReservationAndRead`, `TestHTTPStaleSnapshotRejected`, `TestHTTPCapacityConflictNoSubstitution`, `TestHTTPArriveDepartFlow`
- Gap: B4 (post-denial recovery) is covered by `TestCitizenOwnershipRegression_PostDenialIdempotencyKeySurvives` (already in working tree)

**Rationale:** The coverage map has documentary value but must not masquerade as an executed test.

---

## 5. Summary

| Category | Finding | Severity |
|---|---|---|
| Tokens/credentials in artifacts | None found | — |
| Hardcoded session credentials | None found | — |
| Unsupported PASS labels | `citizen_ownership_regression_test.go` CoverageMap always passes with zero assertions | Medium — cannot be counted as acceptance |
| Unsafe kill-by-port | None found | — |
| npm install in artifacts | None found | — |
| Mutable window test hooks | `?sthira-test-hooks=1` mentioned in comments only | Low — correctly guarded by `import.meta.env.DEV` in source |
| Commented fetch override | In voice-outage-fallback.reticle.js comments only | Informational — manual technique documentation |

---

## 6. Status

**Status: READY_FOR_REVIEW_UNVERIFIED**

One remediation recommended: extract the coverage map documentation from `citizen_ownership_regression_test.go` into `plan/evidence/citizen-ownership-coverage-map.md` and remove the assertion-free `CoverageMap` test function. No secrets found in any artifact. No unsafe instructions requiring correction. PASS labels in `prototype-browser-verification.md` are historically grounded and require no change.

**Exact next action for Opus:** Extract documentation block from `citizen_ownership_regression_test.go` lines 3-36 into a report file; remove or empty the `CoverageMap` function.
