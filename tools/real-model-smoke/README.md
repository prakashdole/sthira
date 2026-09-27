# real-model smoke

`smoke.sh` replays the §7 smoke sequence over HTTP against already-running workers, for a future
paid GPU window, so no code is written while the GPU bills. It does not start the instance, build anything, or edit code.

## Paid-window order (§7 of plan/evidence/real-inference-launch-check.md)

1. §7 step 2 — `GET /health` on asrworker, one `POST /transcribe` per en-IN and hi-IN.
2. §7 step 3 — `GET /health` on middleworker, one `POST /v1/chat/completions`.
3. §7 step 4 — `GET /health` on ttsworker, one `POST /synthesize` (WAV header + checksum).
4. §7 step 5 — one `POST /api/v3/voice/process` (plus `GET /health/live` and `GET /health/ready`, added here; not in §7).

NOT covered by this script — the owner runs these: step 0 instance start and `tools/preflight`; step 1 `docker start sthira-sarvam` plus `curl 127.0.0.1:8000/v1/models`; step 4b (start ttsworker once with `STHIRA_TTS_VOICES_FILE` unset, confirm the adapter reports BLOCKED); §7's second and third middle requests (silent zoom, arrival) and the hi-IN and silent-zoom pipeline variants; step 6 (real browser + microphone).

## Budgets (owner's wall-clock allowance, from the §7 budget column)

0: 10 min · 1: 10 min · 2: 10 min · 3: 15 min · 4: 20 min · 4b: 2 min · 5: 15 min · 6: 20 min. Hard stop: 100 minutes from instance start. Those budgets include worker startup and model load, which is why the script's own `$SMOKE_HTTP_TIMEOUT` (default 30s, per HTTP request) is deliberately far shorter: the owner spends the budget starting workers and loading weights.

## Run (only after §7 steps 0–1 pass and the three workers answer)

```sh
SMOKE_ASR_URL=http://127.0.0.1:8001 SMOKE_MIDDLE_URL=http://127.0.0.1:8002 SMOKE_TTS_URL=http://127.0.0.1:8003 \
SMOKE_BACKEND_URL=http://127.0.0.1:8090 SMOKE_TOKEN="$TOK" SMOKE_WAV_EN=en.wav SMOKE_WAV_HI=hi.wav \
SMOKE_WAV_PIPELINE=en.wav ./smoke.sh
```

Exit 0 all PASS · 1 a FAIL · 2 only BLOCKED. A BLOCKED-only run is never a proof.

## Stop and cleanup (owner, by hand — never part of the script)

- Drain first: `POST /shutdown` to asrworker, ttsworker and middleworker; each returns 200 only after pending requests complete or hit the deadline (`plan/p6-contract.md:323`).
- Then stop the instance (not terminate) and confirm `stopped` read-only (`plan/evidence/real-inference-launch-check.md:207`), using the repository's own documented stop path. Never terminate: it deletes the root volume that holds the weights.
- At 100 minutes, stop regardless of how far the sequence got.

## Evidence to keep

`smoke.sh` stdout, the JSON summary at `$SMOKE_SUMMARY`, and per §7 "Preserve as evidence": commit SHA, served model name from `/v1/models`, per-step PASS/FAIL with HTTP status and 1-2 line excerpts, TTS WAV header fields and checksums (never audio bytes), per-step latency, and the failing request/response shape for any FAIL. Never keep raw logs, recordings, weights or tokens. `voices.DRAFT.json` is a DRAFT: every unverified description is prefixed `DRAFT_REQUIRES_OWNER_REVIEW`, the model card (https://huggingface.co/ai4bharat/indic-parler-tts) is the only permitted source for speaker names and descriptions, and the owner must approve the file before `STHIRA_TTS_VOICES_FILE` may ever name it.
