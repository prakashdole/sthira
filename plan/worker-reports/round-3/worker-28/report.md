# Worker 28 Report — Paid Launch Documentation Delta

**Worker:** worker-28 | **Assignment:** round-3
**Timestamp:** 2026-09-27 | **HEAD:** d95df9e
**Output dir:** plan/worker-reports/round-3/worker-28/

---

## 1. HEAD, Source Hashes, Dirty Files

- **HEAD:** `d95df9e609454877a91b7c82146b3c6368181b17`
- Evidence file SHA-256: `1b834451b1e17c43cf1bd8edf928037eca1fca4833d8909905a19e9c9293ba4d`
- Relevant dirty files: none in scope; eval, audio_mime, styles.css are out-of-scope dirty state

---

## 2. Prior Evidence Inspected

| File | Role |
|------|------|
| `plan/worker-reports/round-2/minimax-j/report.md` | J's 10 findings from source inspection |
| `plan/worker-reports/round-2/minimax-j/launch-checklist.md` | J's corrected launch checklist |
| `plan/evidence/real-inference-launch-check.md` | Authoritative evidence file for this session |

---

## 3. Scope Inspected

- `backend/internal/asrworker/cmd/asrworker/main.go` — confirms `STHIRA_ASR_PYTHON` env var
- `backend/internal/ttsworker/cmd/ttsworker/main.go` — confirms `STHIRA_TTS_PYTHON` env var
- `backend/cmd/sthira-exercise/main.go` — confirms `STHIRA_ASR_URL`, `STHIRA_MIDDLE_URL`, `STHIRA_TTS_URL`, `STHIRA_DATABASE_DSN`, `STHIRA_EXERCISE_SEED`
- `src/sthira_v2/speech_tts_adapter.py` — confirms `STHIRA_TTS_VOICES_FILE` (line 117) and `STHIRA_TTS_DEVICE` default `"cpu"` (line 302)

---

## 4. Assessment: J Findings vs. Evidence File

The evidence file (`real-inference-launch-check.md`) is authoritative for this session. J's checklist is supplementary source-corrected documentation. Most of J's 10 corrections (F1–F10) are already reflected in the evidence file or are design clarifications. Two items represent concrete gaps with operational impact:

### Gap 1: `STHIRA_TTS_VOICES_FILE` absent from step 4

**Evidence:** `speech_tts_adapter.py:117` — if unset, `voices = {}` (zero advertised voices). The worker starts and stays up, but no TTS synthesis is possible. This is a **silent failure mode**.

**Evidence file step 4:** Does not mention `STHIRA_TTS_VOICES_FILE`. The command passes `STHIRA_TTS_DEVICE=cuda` and `STHIRA_TTS_SYNTHETIC_EXERCISE=1` but omits the voices file.

**Impact:** An operator following step 4 would not know to set `STHIRA_TTS_VOICES_FILE`, and would see a TTS worker that starts cleanly but advertises zero voices.

### Gap 2: `sthira-exercise` startup command omits required vars

**Evidence:** `sthira-exercise/main.go:253` — `STHIRA_DATABASE_DSN` is required, binary logs a fatal error and exits without it. `main.go:273` — `STHIRA_EXERCISE_SEED=1` is required on first boot to seed demo data.

**Evidence file step 5:** Says "local `sthira-exercise` with `STHIRA_{ASR,MIDDLE,TTS}_URL + _TOKEN=$TOK`" — vague, and omits both `STHIRA_DATABASE_DSN` and `STHIRA_EXERCISE_SEED`.

**Impact:** Without `STHIRA_DATABASE_DSN`, the exercise server exits immediately with a log error. Without `STHIRA_EXERCISE_SEED=1`, the database is seeded but no demo data is present (step 5 would return empty results).

---

## 5. What Is NOT a Gap (Already in Evidence File)

| J Finding | Evidence File Status |
|-----------|-------------------|
| F1: PYTHONPATH is shell-inherited, not Go-read | Already correct in evidence file §7 |
| F2: Separate `STHIRA_TTS_PYTHON` | Evidence file uses `$PY` in step 4 which maps to the same binary; functionally equivalent |
| F5: `STHIRA_TTS_DEVICE` defaults to `"cpu"` | Evidence file step 4 correctly passes `DEVICE=cuda` |
| F7: vLLM port 8000 loopback | Confirmed correct |
| F8: SarvamModelID confirmed | Confirmed correct |
| F9: max_tokens = 512 | Evidence file §7 explicitly states `max_tokens 512` |

---

## 6. Documentation Patch

**File:** `plan/worker-reports/round-3/worker-28/evidence-documentation.patch`
**Status:** APPLIED directly to `plan/evidence/real-inference-launch-check.md` (edit tool, 2026-09-27)
**Base SHA-256:** `1b834451b1e17c43cf1bd8edf928037eca1fca4833d8909905a19e9c9293ba4d`
**Post-apply SHA-256:** `db8bd4c461ad4e612923de608f4904b06b75ecd71f212af08394467f5259e3dc`

Two changes applied to the evidence file:

1. **New step 4b** — explicit `STHIRA_TTS_VOICES_FILE` check: zero voices when unset, voices advertised when set. Makes the silent failure mode visible in the smoke sequence.

2. **Step 5 explicit startup command** — replaces vague "`sthira-exercise` with `STHIRA_{ASR,MIDDLE,TTS}_URL + _TOKEN=$TOK`" with the full command including `STHIRA_EXERCISE_SEED=1`, `STHIRA_DATABASE_DSN="postgres://user:pass@127.0.0.1:5432/sthira"`, and all six URL/TOKEN vars. The `postgres://...` value is a placeholder to be replaced by the operator.

---

## 7. Integration Risks

- **No source code changes** — documentation only
- **No AWS/SSH/inference** — patch is text; verification deferred to authorized session
- The `STHIRA_DATABASE_DSN` value in the patch is a placeholder (`postgres://...`) — the operator must supply the actual connection string. The patch does not invent or guess this value.
- `STHIRA_EXERCISE_SEED=1` is noted for first boot only; subsequent restarts should omit it (reseeding non-idempotent, fails closed)

---

## 8. Verification

**Source inspection:** Performed (asrworker/main.go, ttsworker/main.go, sthira-exercise/main.go, speech_tts_adapter.py)
**Execution tests:** NOT_RUN — deferred by user; requires authorized GPU session

The patch has already been applied. Confirm the current state:

```bash
# Verify step 4b and step 5 are correct
git diff plan/evidence/real-inference-launch-check.md

# Confirm step 4b mentions STHIRA_TTS_VOICES_FILE
# Confirm step 5 shows DATABASE_DSN, EXERCISE_SEED, all six URL/TOKEN vars
```

---

## 9. Status

**STATUS: APPLIED — PENDING GPU AUTHORIZATION**

The two documented gaps have been patched directly into `plan/evidence/real-inference-launch-check.md`:
- Step 4b added (TTS voices file — silent failure if unset)
- Step 5 made explicit (DATABASE_DSN, EXERCISE_SEED, all six worker URL/TOKEN vars)

The `postgres://user:pass@127.0.0.1:5432/sthira` placeholder in step 5 must be replaced by the operator with the actual connection string. Remaining verification requires authorized GPU session; no local checks substitute for live inference.
