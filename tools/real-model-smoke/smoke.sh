#!/usr/bin/env bash
# real-model smoke: the plan/evidence/real-inference-launch-check.md §7 order,
# HTTP-only, so no code is written while the GPU is billing.
#
# Read-only against every worker: GET /health plus the §7 probe requests. No
# request is ever retried (urllib does not retry, and no retry flag exists in
# this file) because POST /transcribe, POST /v1/chat/completions,
# POST /synthesize and POST /api/v3/voice/process are non-idempotent writes.
#
# Secrets come from the environment only: this file contains no token, URL
# credential or key. The JSON summary records step, verdict, HTTP status,
# elapsed ms and a short detail string — never audio bytes, never a token.
#
# Usage:  SMOKE_ASR_URL=http://127.0.0.1:8001 \
#         SMOKE_MIDDLE_URL=http://127.0.0.1:8002 \
#         SMOKE_TTS_URL=http://127.0.0.1:8003 \
#         SMOKE_BACKEND_URL=http://127.0.0.1:8090 \
#         SMOKE_TOKEN="$TOK" \
#         SMOKE_WAV_EN=en.wav SMOKE_WAV_HI=hi.wav SMOKE_WAV_PIPELINE=en.wav \
#         ./smoke.sh
#
# Exit codes: 0 all PASS; 1 at least one FAIL (stops at the first FAIL, §7);
# 2 only BLOCKED steps (an incomplete run is never a proof).
set -uo pipefail

PY="${SMOKE_PY:-python3}"
command -v "$PY" >/dev/null 2>&1 || { echo "FAIL  preflight: $PY not found" >&2; exit 1; }

exec "$PY" - <<'PYEOF'
import base64, hashlib, json, os, sys, time, urllib.error, urllib.request, wave

# Routes and request shapes are taken from the Go source, not from memory:
#   asrworker  GET  /health                 backend/internal/asrworker/server.go:97
#              POST /transcribe             backend/internal/asrworker/server.go:98, body server.go:265-273
#   middle     GET  /health                 backend/internal/middleworker/server.go:119
#              POST /v1/chat/completions    backend/internal/middleworker/server.go:120, body wire.go:111-117
#   ttsworker  GET  /health                 backend/internal/ttsworker/server.go:61
#              POST /synthesize             backend/internal/ttsworker/server.go:62, body worker.go:27-41
#   backend    GET  /health/live|/health/ready   backend/internal/httpserver/server.go:333-334
#              POST /api/v3/voice/process   backend/internal/httpserver/voice_process.go:81,166-212
#                                         body contracts/pipeline.go:62-69
# Bearer auth on the three workers when a token is configured:
#   asrworker/server.go:95,199-221 · middleworker/server.go:157-168 ·
#   ttsworker/server.go:232-248
#
# The demo template text below is the exact string ttsworker pre-generates at
# startup (backend/internal/ttsworker/cmd/ttsworker/main.go:64-74), so a real
# cache hit needs this byte-identical text, its SHA-256, template_version 1 and
# the source version the worker clock reports on /health (worker.go:190).
TEMPLATE_TEXT = {
    ("en-IN", "destination_options"): "Destination choices are displayed on screen.",
    ("hi-IN", "destination_options"): "गंतव्य विकल्प स्क्रीन पर प्रदर्शित हैं।",
}

E = os.environ.get
TIMEOUT = float(E("SMOKE_HTTP_TIMEOUT", "30"))
DATA_VERSION = E("SMOKE_EXPECT_DATA_VERSION", "PKGDEMO-1:1")
JURISDICTION = E("SMOKE_JURISDICTION", "DEMO-EXERCISE")
TOKEN = E("SMOKE_TOKEN", "")
BACKEND_TOKEN = E("SMOKE_BACKEND_TOKEN", "")
TTS_LANG = E("SMOKE_TTS_LANGUAGE", "en-IN")
TTS_KEY = E("SMOKE_TTS_SPEECH_KEY", "destination_options")
MID_LANG = E("SMOKE_MIDDLE_LANGUAGE", "en-IN")
MID_TEXT = E("SMOKE_MIDDLE_TEXT", "Meppadi")
SUMMARY = E("SMOKE_SUMMARY", os.path.join(os.getcwd(), "smoke-summary.json"))

STEPS, STOP = [], False


def record(name, verdict, http, ms, detail):
    STEPS.append({"step": name, "verdict": verdict, "http_status": http,
                  "elapsed_ms": ms, "detail": detail[:200]})
    print("%-7s %-22s http=%-4s ms=%-6s %s" % (verdict, name, http, ms, detail[:200]), flush=True)


def call(method, url, token, body=None):
    """One attempt. Returns (status, elapsed_ms, decoded_json_or_None, error)."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    started = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT) as resp:
            raw, status = resp.read(4 * 1024 * 1024), resp.status
    except urllib.error.HTTPError as exc:          # a 4xx/5xx still carries a body
        raw, status = exc.read(4096), exc.code
    except Exception as exc:                       # timeout, refused, reset
        return 0, int((time.monotonic() - started) * 1000), None, type(exc).__name__ + ": " + str(exc)
    ms = int((time.monotonic() - started) * 1000)
    try:
        return status, ms, json.loads(raw), None
    except Exception:
        return status, ms, None, "non-JSON response (%d bytes)" % len(raw)


def wav_fields(raw):
    """(sample_rate, channels, bit_depth, frames) from RIFF bytes, or raises."""
    import io
    with wave.open(io.BytesIO(raw), "rb") as wf:
        return wf.getframerate(), wf.getnchannels(), wf.getsampwidth() * 8, wf.getnframes()


def wav_request(path, lang, rid):
    """contracts.PipelineRequest (contracts/pipeline.go:62-69) for one call."""
    with open(path, "rb") as fp:
        blob = fp.read()
    return rid, {"request_id": rid, "jurisdiction": JURISDICTION, "language": lang,
                 "input": {"kind": "audio", "content_type": "audio/wav",
                           "body_b64": base64.b64encode(blob).decode()},
                 "render": {"kind": "tts"}}


def step(name, fn):
    global STOP
    if STOP:
        record(name, "SKIPPED", 0, 0, "not run: sequence stopped at the first FAIL (§7)")
        return
    verdict, http, ms, detail = fn()
    record(name, verdict, http, ms, detail)
    if verdict == "FAIL":
        STOP = True


# ---------------------------------------------------------------- 0. preflight
missing = [v for v in ("SMOKE_ASR_URL", "SMOKE_MIDDLE_URL", "SMOKE_TTS_URL", "SMOKE_BACKEND_URL")
           if not E(v)]
blocked = {}
for tag, var in (("asr-en-IN", "SMOKE_WAV_EN"), ("asr-hi-IN", "SMOKE_WAV_HI"),
                 ("pipeline", "SMOKE_WAV_PIPELINE")):
    path = E(var)
    if not path:
        blocked[tag] = "%s not set" % var
    elif not os.path.isfile(path):
        blocked[tag] = "%s is not a file: %s" % (var, path)
if missing:
    record("preflight-urls", "FAIL", 0, 0, ", ".join(missing))
    STOP = True
for name, why in blocked.items():
    record(name, "BLOCKED", 0, 0, why)
if missing:
    STOP = True
if not missing:
    record("preflight", "PASS", 0, 0,
           "timeout=%gs data_version=%s" % (int(TIMEOUT), DATA_VERSION))

# ---------------------------------------------------------- 1. ASR health (P6)
def asr_health():
    status, ms, body, err = call("GET", E("SMOKE_ASR_URL").rstrip("/") + "/health", TOKEN)
    if err:
        return "FAIL", status, ms, err
    langs = (body or {}).get("supported_languages") or []
    want = [l for l in ("en-IN", "hi-IN") if l not in langs]
    ok = status == 200 and body.get("ready") and not want
    return ("PASS" if ok else "FAIL"), status, ms, (
        "ready=%s warm=%s languages=%s%s" % (body.get("ready"), body.get("warm"), ",".join(langs),
                                            " missing=" + ",".join(want) if want else ""))


def asr_transcribe(lang, wav_var):
    """POST /transcribe with the body asrworker/server.go:265-273 declares."""
    def run():
        if lang in blocked:
            return "BLOCKED", 0, 0, blocked[lang]
        path = E(wav_var)
        with open(path, "rb") as fp:
            blob = fp.read()
        rid = "smoke-asr-" + lang
        status, ms, body, err = call("POST", E("SMOKE_ASR_URL").rstrip("/") + "/transcribe", TOKEN, {
            "request_id": rid, "language": lang, "content_type": "audio/wav",
            "audio_b64": base64.b64encode(blob).decode(), "byte_size": len(blob),
            "decoded_seconds": 0.0, "deadline_ms": int(TIMEOUT * 1000)})
        if err:
            return "FAIL", status, ms, err
        state, text = body.get("state"), body.get("text") or ""
        ok = status == 200 and state == "OK" and text.strip() != "" and body.get("language") == lang
        return ("PASS" if ok else "FAIL"), status, ms, (
            "state=%s language=%s text_chars=%d revision=%s" % (state, body.get("language"), len(text),
                                                              body.get("model_revision") or "-"))
    return run


# ------------------------------------------------------ 2. middle health + request
def middle_health():
    status, ms, body, err = call("GET", E("SMOKE_MIDDLE_URL").rstrip("/") + "/health", TOKEN)
    if err:
        return "FAIL", status, ms, err
    ok = status == 200 and body.get("ready")
    return ("PASS" if ok else "FAIL"), status, ms, (
        "ready=%s warm=%s models=%s" % (body.get("ready"), body.get("warm"),
                                         ",".join(m.get("model_id", "?") for m in body.get("models") or [])))


def middle_propose():
    """POST /v1/chat/completions; envelope + scoped context per middleworker/wire.go:29-117."""
    rid = "smoke-middle-" + MID_LANG
    status, ms, body, err = call("POST", E("SMOKE_MIDDLE_URL").rstrip("/") + "/v1/chat/completions", TOKEN, {
        "request_id": rid,
        "scoped_context": {
            "request_id": rid, "data_version": DATA_VERSION, "schema_version": "3.0",
            "jurisdiction": JURISDICTION, "allowed_languages": [MID_LANG],
            "template_keys": ["destination_options"], "issued_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())},
        "transcript": {"request_id": "smoke-asr-" + MID_LANG, "language": MID_LANG,
                       "text": MID_TEXT, "state": "OK"},
        "max_output_tokens": 512, "deadline_ms": int(TIMEOUT * 1000)})
    if err:
        return "FAIL", status, ms, err
    prop = body.get("proposal") or {}
    ok = (status == 200 and prop.get("request_id") == rid and prop.get("data_version") == DATA_VERSION
          and prop.get("status") and prop.get("language") == MID_LANG)
    return ("PASS" if ok else "FAIL"), status, ms, (
        "status=%s schema_version=%s language=%s actions=%d model_revision=%s" % (
            prop.get("status"), prop.get("schema_version"), prop.get("language"),
            len(prop.get("actions") or []), body.get("model_revision") or "-"))


# ------------------------------------------------------------ 3. TTS health + synth
def tts_health():
    status, ms, body, err = call("GET", E("SMOKE_TTS_URL").rstrip("/") + "/health", TOKEN)
    if err:
        return "FAIL", status, ms, err
    voices = [v.get("name") for v in body.get("runtime_voices") or []]
    ok = status == 200 and body.get("ready")
    return ("PASS" if ok else "FAIL"), status, ms, (
        "ready=%s warm=%s source_version=%s voices=%d %s" % (
            body.get("ready"), body.get("warm"), body.get("current_source_version"), len(voices),
            "(adapter BLOCKED: no approved voices file?)" if not body.get("ready") else ""))


def tts_synthesize():
    """POST /synthesize; body per ttsworker/worker.go:27-41, cache-only hot path."""
    text = E("SMOKE_TTS_TEXT") or TEMPLATE_TEXT.get((TTS_LANG, TTS_KEY))
    if text is None:
        return "BLOCKED", 0, 0, "no known text for %s/%s; set SMOKE_TTS_TEXT" % (TTS_LANG, TTS_KEY)
    # The worker overrides sample_rate with its native rate (worker.go:452-455), so
    # the caller's value is not part of the cache identity.
    status, ms, body, err = call("POST", E("SMOKE_TTS_URL").rstrip("/") + "/synthesize", TOKEN, {
        "request_id": "smoke-tts-" + TTS_LANG, "speech_key": TTS_KEY, "language": TTS_LANG,
        "text": text, "source_version": int(E("SMOKE_TTS_SOURCE_VERSION", "1")),
        "template_version": 1, "template_sha256": hashlib.sha256(text.encode()).hexdigest(),
        "settings": {"sample_rate": 0, "bit_depth": 16, "channels": 1},
        "deadline_ms": int(TIMEOUT * 1000)})
    if err:
        return "FAIL", status, ms, err
    if status != 200 or body.get("state") != "OK":
        return "FAIL", status, ms, "state=%s reason=%s" % (body.get("state"), body.get("reason") or "-")
    raw = base64.b64decode(body.get("audio_b64") or "")
    digest = hashlib.sha256(raw).hexdigest()
    declared = (body.get("settings") or {}).get("sample_rate")
    try:
        rate, channels, bits, frames = wav_fields(raw)
    except Exception as exc:
        return "FAIL", status, ms, "audio_b64 is not a readable WAV: %s" % exc
    ok = digest == body.get("checksum_sha256") and rate == declared
    return ("PASS" if ok else "FAIL"), status, ms, (
        "cache_hit=%s wav_rate=%d declared_rate=%d ch=%d bits=%d frames=%d bytes=%d sha256=%s checksum_match=%s" % (
            body.get("cache_hit"), rate, declared, channels, bits, frames, len(raw), digest,
            digest == body.get("checksum_sha256")))


# ------------------------------------------------------- 4. backend health + pipeline
def backend_live():
    # Both health routes answer in the /api/v3 envelope; the report is under "data".
    status, ms, body, err = call("GET", E("SMOKE_BACKEND_URL").rstrip("/") + "/health/live", BACKEND_TOKEN)
    if err:
        return "FAIL", status, ms, err
    live = (body or {}).get("data") or {}
    ok = status == 200 and live.get("status") == "LIVE"
    return ("PASS" if ok else "FAIL"), status, ms, "status=%s" % live.get("status", "-")


def backend_ready():
    status, ms, body, err = call("GET", E("SMOKE_BACKEND_URL").rstrip("/") + "/health/ready", BACKEND_TOKEN)
    if err:
        return "FAIL", status, ms, err
    report = (body or {}).get("data") or {}
    subs = {k: v.get("status") for k, v in (report.get("subsystems") or {}).items()}
    ok = status == 200 and report.get("status") == "READY"
    return ("PASS" if ok else "FAIL"), status, ms, "status=%s subsystems=%s" % (
        report.get("status"), ",".join("%s=%s" % kv for kv in sorted(subs.items())))


def pipeline():
    if "pipeline" in blocked:
        return "BLOCKED", 0, 0, blocked["pipeline"]
    rid, req = wav_request(E("SMOKE_WAV_PIPELINE"), E("SMOKE_PIPELINE_LANGUAGE", "en-IN"), "smoke-pipeline")
    status, ms, body, err = call("POST", E("SMOKE_BACKEND_URL").rstrip("/") + "/api/v3/voice/process",
                                 BACKEND_TOKEN, req)
    if err:
        return "FAIL", status, ms, err
    data = body.get("data") or {}
    codes = ",".join(e.get("code", "") for e in body.get("errors") or [])
    ok = (status == 200 and data.get("state") == "OK"
          and data.get("data_version") == DATA_VERSION and not data.get("stage_failures"))
    detail = "state=%s data_version=%s template=%s stage_failures=%s errors=%s" % (
        data.get("state"), data.get("data_version"), (data.get("template") or {}).get("speech_key") or "-",
        ",".join(data.get("stage_failures") or []) or "-", codes or "-")
    audio = data.get("audio")
    if audio:
        raw = base64.b64decode(audio.get("audio_b64") or "")
        try:
            rate, channels, bits, _ = wav_fields(raw)
        except Exception as exc:
            return "FAIL", status, ms, detail + " audio is not a readable WAV: %s" % exc
        match = hashlib.sha256(raw).hexdigest() == audio.get("checksum_sha256")
        ok = ok and match and rate == (audio.get("settings") or {}).get("sample_rate")
        detail += (" audio: lang=%s rate=%d declared=%d bytes=%d sha256=%s checksum_match=%s" % (
            audio.get("language"), rate, (audio.get("settings") or {}).get("sample_rate"), len(raw),
            hashlib.sha256(raw).hexdigest(), match))
    else:
        detail += " audio: none (expected for a speech_key-less proposal)"
    return ("PASS" if ok else "FAIL"), status, ms, detail


# ------------------------------------------------------------------- the §7 order
if not STOP:
    step("asr-health", asr_health)
    step("asr-en-IN", asr_transcribe("en-IN", "SMOKE_WAV_EN"))
    step("asr-hi-IN", asr_transcribe("hi-IN", "SMOKE_WAV_HI"))
    step("middle-health", middle_health)
    step("middle-request", middle_propose)
    step("tts-health", tts_health)
    step("tts-request", tts_synthesize)
    step("backend-live", backend_live)
    step("backend-ready", backend_ready)
    step("pipeline-process", pipeline)

counts = {v: sum(1 for s in STEPS if s["verdict"] == v) for v in ("PASS", "FAIL", "BLOCKED", "SKIPPED")}
with open(SUMMARY, "w", encoding="utf-8") as fp:
    json.dump({"generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
               "expect_data_version": DATA_VERSION, "counts": counts, "steps": STEPS}, fp, indent=2)
    fp.write("\n")
print("summary: %s  %s" % (SUMMARY, json.dumps(counts)))
sys.exit(1 if counts["FAIL"] else (2 if counts["BLOCKED"] else 0))
PYEOF
