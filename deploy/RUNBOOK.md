# Sthira v2 demo runbook

The demo profile is synthetic only. It must not be changed to `SHADOW`, `PILOT`,
or `PRODUCTION` without the source, database, authentication, security,
language, accessibility, and government-ownership gates recorded in `prompt.md`.

## Hackathon local presentation

Open two terminals from the repository root:

```sh
# Terminal 1 — synthetic API, local models, and optional Azure map wording
./.venv/bin/python -m uvicorn sthira.api.app:app --host 127.0.0.1 --port 8000
```

```sh
# Terminal 2 — citizen interface
cd frontend/v2
npm run dev
```

Open `http://127.0.0.1:5173/`. The first local TTS play can take about 28
seconds while Indic Parler-TTS loads; repeat playback is cached in the running
API process. In English, use **Demo transcript** or visible map controls;
microphone capture is limited to Hindi and Malayalam by this local ASR artifact.

## Local model-enabled demo

The default container does not include inference dependencies or model weights.
To run the local AI4Bharat speech adapter, build only on an approved model host:

```sh
docker compose -f deploy/docker-compose.demo.yml build --build-arg INCLUDE_LOCAL_VOICE_RUNTIME=true
STHIRA_ASR_MODEL_HOST_DIR=/absolute/path/to/indic-conformer-600m-multilingual \
STHIRA_TTS_MODEL_HOST_DIR=/absolute/path/to/indic-parler-tts \
docker compose -f deploy/docker-compose.demo.yml up
```

The speech weights are mounted read-only. Raw voice bytes are used only for the
bounded transcription request and are not retained. Azure credentials, if used
for the synthetic map interpreter, belong only in an ignored local `.env`.

## Failure handling

- If the scenario API fails, keep the citizen screen at the verification/error
  state. Do not display cached guidance as current.
- If satellite imagery fails, retain local GeoJSON overlays, text route steps,
  and the synthetic disclaimer. The imagery is visual context only; do not
  infer operational facts from it.
- If speech fails or confidence is low, keep touch and keyboard controls active
  and apply no map action.
- If an emergency call is requested, show confirmation and open only the device
  dialler after explicit action. Never claim connection or dispatch.
- If a cached alert or route expires or is cancelled, invalidate it and show the
  unavailable/stale state.

## Recovery evidence required before activation

PostgreSQL 16/PostGIS migration and restore rehearsal, source artifact recovery,
authorized government samples, speech benchmark results, approved language/ISL
review, security/privacy assessment, and pilot-owner sign-off are external gates.
