# Real-language collection and review requirements

This directory describes the *consent, sourcing, and reviewer* gate for
real-language recording. **Until these are satisfied, real-language
cases do not exist and the harness produces `NOT_EVALUATED` for any
case whose `Provenance.Kind` is `CONSENTED`.** The absence of cases is
not a zero-failure or 100%-success signal — see O03 / O11 in
`plan/open-decisions.md`.

## Why this is its own document

The synthetic suite (`cases/synthetic/`) exercises the *shape* of the
contract: every category, every dialect code-switch, every status
code. Real-language cases will exercise the *quality* of the live
models — confusion, hallucination, and honest silence. Mixing the two
in one suite (or pretending the synthetic suite stands in for real
traffic) creates the kind of false-green the harness exists to
prevent.

A real case MUST satisfy all of the following before being loaded by
`eval-run -real`. The requirements come from O03 (`Regional
service-language/dialect matrix including English, ASR/TTS artifact
licenses and human reviewers`) and O11 (`Approved instruction
translations/ISL corpus and qualified review partners`).

## Provenance

Every real case carries a `Provenance` object:

```json
{
  "provenance": {
    "kind": "CONSENTED",
    "source": "<agency / dataset name>",
    "license": "<SPDX-ish identifier>",
    "consent_id": "<external reference>",
    "recorded_at": "<ISO8601>",
    "notes": "<short, no PII>"
  }
}
```

`kind=CONSENTED` cases are NOT committed to source. They are
discovered from an out-of-repo manifest:

```sh
go run ./cmd/eval-run -real \
    -suite ./cases/synthetic \
    -real-manifest ./outside-repo/manifest.json \
    -asr http://localhost:7101 \
    -middle http://localhost:7201 \
    -tts http://localhost:7301 \
    -bearer "$WORKER_BEARER"
```

The manifest pairs case IDs with their on-disk audio path (and
sha256) and is the only way the runner accepts recordings. Anything
else stays synthetic.

## Recording requirements (per region / language / cohort)

| Field | Requirement |
|-------|-------------|
| Recording format | 16-bit PCM mono WAV at the language's native sample rate (8 kHz – 48 kHz per `plan/parameters.md`) |
| Max duration | 20 s (`plan/parameters.md`) |
| Bounded compressed size | 512 KiB (`contracts.MaxTranscriptionCompressedBytes`) |
| Native script | text must be in the language's own script; transliteration is allowed only as a parallel transcript |
| Sample diversity | at least 5 distinct speakers per `language × cohort × category` cell, mixed-gender where culturally appropriate |
| Cohort coverage | ADULT only in v1; CHILD is BLOCKED until a separate O10 + O11 child-consent program lands. ELDER is permitted only with explicit reviewer note |
| Ambient noise floor | recorded at ≤ 45 dB SNR for the clean band; a `NOISY_AUDIO` series is permitted to be lower and is captured under Category `NOISY_AUDIO` |
| Clipped series | a separate `CLIPPED_AUDIO` series truncates at known timestamps (0.4s, 0.8s of a 1.2s utterance) |

### Consent

Every speaker signs a consent that:

1. allows storage of the audio + transcript for the duration of the
   project's quality program;
2. permits the audio's hash to appear in audit trails and benchmarks;
3. acknowledges that, if the audio is withdrawn, the case moves from
   `READY` to `NEEDS_REVIEW` in the runner's bookkeeping — and that
   the runner refuses to evaluate a case whose consent was withdrawn
   but hasn't been reset;
4. prevents the audio from being committed to source. The audio only
   ever lives outside the repo. Only the case ID and the transcript
   appear in fixtures; the audio reference is a path.

### Reviewer roles (per O11)

| Role | Responsibility |
|------|----------------|
| Linguistic reviewer | confirms the transcript is faithful, completes sentences, and is in the language's own script |
| Content reviewer | confirms no fabricated emergency wording, no ISL/English backstop inserted without explicit reason |
| Domain reviewer | confirms the case's Category label is honest (e.g. a noisy recording is `NOISY_AUDIO`, not `AMBIGUOUS_LOCALITY`) |
| Accessibility reviewer | confirms the case passes screen-reader feedback where one was captured |

A case with NO reviewer signatures is `NEEDS_REVIEW` and will not
load into the runner.

### Translation approval (ISL, English backstop)

ISL content and English translation fall under O11. The corpus MUST
NOT ship any string that was auto-translated without a qualified human
sign-off. This directory rejects such translations at the schema
layer: every `SYNTHETIC` case carries the literal string the
deterministic rule returns; `CONSENTED` cases carry an approved
English transcript or a `linguistic_reviewer_id` annotation.

### Withdrawal protocol

If a contributor withdraws consent:

1. The manifest entry moves to `status: WITHDRAWN`.
2. The runner logs the case at `NOT_EVALUATED` (`Reason: WITHDRAWN`).
3. The case remains in the manifest for audit and is never deleted.

## Where real cases live

| Path | Purpose | Lives in Git? |
|------|---------|---------------|
| `cases/synthetic/suite.json` | Synthetic representative cases | yes |
| `cases/real/*` | Real recorded audio references | NO — outside-repo manifest only |
| `cmd/eval-run` | Runner that consumes both | yes |

## Run outcomes when collection is incomplete

When there are fewer than `MinSampleForClaim` real cases for a
language, the report for that language reads:

```
### ml-IN (n=2, pass=N/A, fail=N/A, ne=2)
  cohort: ADULT_SYNTHETIC
  NOT_EVALUATED (n < 5)
```

The reporter never rounds up to a fraction, never invents data, and
never reports `100%` or `0%`. The total run report card explicitly
states:

> Missing reviewers/samples yields `NOT_EVALUATED`, not zero or 100%.

This is documented in the prompt and must remain in the report.
