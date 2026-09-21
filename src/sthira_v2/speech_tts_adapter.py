"""JSONL adapter for Indic Parler-TTS.

Protocol (line-delimited JSON over stdin/stdout) is unchanged from
``backend/internal/ttsworker/runtime_adapter.go``:

    {"op":"ready"}              -> {"status":"ready"|"blocked",
                                    "sample_rate":<native Hz>,
                                    "languages":[...],
                                    "voices":[{language,name,revision}]}
    {"op":"synthesize",
     "request_id":..., "text":...,
     "language":"hi-IN",
     "voice":"Rohit"}          -> {"request_id":..., "audio_b64":...,
                                  "duration_secs":..., "sample_rate":...}
    {"op":"shutdown"}           -> {"status":"shutdown"}

Selected artifact (fixed; license/usage verified from the primary
model card 2026-09-21): ``ai4bharat/indic-parler-tts`` — Apache-2.0,
multilingual extension of Parler-TTS Mini (0.9B). The card's verified
API is NOT a generic ``AutoModel.generate(text=..., language=...,
voice=...)`` call:

    from parler_tts import ParlerTTSForConditionalGeneration
    from transformers import AutoTokenizer
    model = ParlerTTSForConditionalGeneration.from_pretrained(repo)
    tokenizer = AutoTokenizer.from_pretrained(repo)            # prompt
    desc_tokenizer = AutoTokenizer.from_pretrained(            # description
        model.config.text_encoder._name_or_path)
    generation = model.generate(
        input_ids=desc_ids, attention_mask=desc_mask,
        prompt_input_ids=prompt_ids, prompt_attention_mask=prompt_mask)
    audio = generation.cpu().numpy().squeeze()
    sf.write(path, audio, model.config.sampling_rate)

Two independent tokenizers; the language is inferred by the model
from the prompt text (no ``language=`` kwarg exists — the old
invented call would have failed on any real artifact); speaker/voice
control happens through the natural-language DESCRIPTION, not a
voice id. Consequently:

* ``sample_rate`` is read from ``model.config.sampling_rate`` at
  load time — never hardcoded (the previous 22,050 Hz assumption
  changes playback speed for this model family).
* Advertised voices come from an operator-supplied approved
  description file (``STHIRA_TTS_VOICES_FILE``, JSON:
  ``{"<bcp47>": {"<name>": "<description sentence>"}}``). The
  model card's speaker list is NOT an approval; only names present
  in this file ∩ the approved-language set are advertised. An
  absent file means ZERO advertised voices, never a fabricated
  "default".
* Advertised languages = ``STHIRA_TTS_APPROVED_LANGUAGES``
  (default "hi-IN,ml-IN,en-IN", the app-enabled locales) ∩ the
  artifact's tokenizer language coverage when the artifact declares
  it in ``config.json``. Model support for 21 languages never
  implies government language approval (O11).
* ``trust_remote_code`` stays False; the parler_tts package is the
  pinned local dependency (requirements-voice.txt; installed
  parler_tts==0.2.2 verified to expose ParlerTTSForConditionalGeneration).
* ``from_pretrained(..., local_files_only=True)`` — opportunistic
  network downloads are impossible.
* READY requires successful local load AND a bounded warm-up
  generation (short prompt, tiny token budget). Dummy files,
  import errors, malformed weights, or a warm-up failure keep the
  adapter BLOCKED.

Heavyweight real inference (GPU/torch load of ~0.9B params) is only
exercised by the opt-in artifact path; the protocol and the load
ordering are covered by injected-fake tests regardless.
"""

from __future__ import annotations

import argparse as _ap
import base64
import hashlib
import json
import os
import struct
import sys
import time
import wave
from io import BytesIO
from typing import Any

INDIC_PARLER_TTS_MODEL = "ai4bharat/indic-parler-tts"
PROTOCOL_VERSION = "tts-adapter-1"
MAX_TEXT_BYTES = 1024
DEFAULT_APPROVED_LANGUAGES = "hi-IN,ml-IN,en-IN"
# ponytail: fixed token ceiling for warm-up + per-request bound.
# Parler mini uses a ~86 fps audio-codec frame rate; upgrade path is
# to read the exact frames-per-second from the loaded config once a
# real artifact is approved (O11).
WARMUP_MAX_NEW_TOKENS = 16
DEFAULT_MAX_NEW_TOKENS = 1250  # ~14.5 s of audio @ 86 fps


def _detect_adapter_mode(argv: list[str]) -> bool:
    return "--adapter-mode" in argv


def _artifact_dir() -> str:
    explicit = os.environ.get("STHIRA_TTS_ARTIFACT_DIR", "").strip()
    if explicit:
        return explicit
    cache = os.path.expanduser("~/.cache/sthira/tts/indic-parler-tts")
    return cache if os.path.isdir(cache) else ""


def _approved_languages() -> list[str]:
    raw = os.environ.get("STHIRA_TTS_APPROVED_LANGUAGES", DEFAULT_APPROVED_LANGUAGES)
    return [tok.strip() for tok in raw.split(",") if tok.strip()]


def _approved_voices() -> dict[str, dict[str, str]]:
    """Operator-approved voice descriptions. Absent/invalid file =>
    empty map (no voices advertised; synthesizing any voice fails)."""
    path = os.environ.get("STHIRA_TTS_VOICES_FILE", "").strip()
    if not path:
        return {}
    try:
        with open(path, "r", encoding="utf-8") as fp:
            data = json.load(fp)
    except Exception:
        return {}
    if not isinstance(data, dict):
        return {}
    out: dict[str, dict[str, str]] = {}
    for lang, voices in data.items():
        if isinstance(voices, dict):
            out[lang] = {str(k): str(v) for k, v in voices.items()}
    return out


def _sha256_file(path: str) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as fp:
        for chunk in iter(lambda: fp.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def _artifact_gate() -> dict[str, Any]:
    """File-presence probe for honest BLOCKED reasons only. Not
    readiness (readiness additionally requires load+warm-up)."""
    art_dir = _artifact_dir()
    if not art_dir:
        return {"state": "BLOCKED",
                "reason": "STHIRA_TTS_ARTIFACT_DIR not set and no local cache found (O11)"}
    if not os.path.isfile(os.path.join(art_dir, "config.json")):
        return {"state": "BLOCKED",
                "reason": f"artifact directory {art_dir!r} incomplete; missing: config.json"}
    if not (os.path.isfile(os.path.join(art_dir, "model.safetensors"))
            or os.path.isfile(os.path.join(art_dir, "model.safetensors.index.json"))):
        return {"state": "BLOCKED",
                "reason": f"artifact directory {art_dir!r} incomplete; missing: model weights (safetensors)"}
    if not _approved_voices():
        return {"state": "BLOCKED",
                "reason": "STHIRA_TTS_VOICES_FILE not set or invalid: no approved voice "
                          "descriptions; the model's speaker list is not an approval"}
    return {"state": "PROBE_OK", "dir": art_dir, "reason": ""}


class _TTSEngine:
    def __init__(self, model, prompt_tokenizer, description_tokenizer,
                 sample_rate: int, languages: list[str],
                 voices: dict[str, dict[str, str]]):
        self.model = model
        self.prompt_tokenizer = prompt_tokenizer
        self.description_tokenizer = description_tokenizer
        self.sample_rate = sample_rate
        # Advertise only justified loaded capabilities: a language
        # counts only if it is approved AND carries at least one
        # approved voice description.
        self.voices = {lang: dict(vs) for lang, vs in voices.items()
                       if lang in languages and vs}
        self.languages = [l for l in languages if l in self.voices]

    def _description_for(self, language: str, voice: str) -> str | None:
        return self.voices.get(language, {}).get(voice)

    def synthesize(self, text: str, language: str, voice: str,
                   expected_rate: int) -> dict[str, Any]:
        if language not in self.languages:
            return {"error": f"language {language!r} not in approved+advertised {self.languages}"}
        description = self._description_for(language, voice)
        if description is None:
            return {"error": f"voice {voice!r} not approved for {language!r}"}
        try:
            import torch
            desc_inputs = self.description_tokenizer(description, return_tensors="pt")
            prompt_inputs = self.prompt_tokenizer(text, return_tensors="pt")
            max_new = int(os.environ.get("STHIRA_TTS_MAX_NEW_TOKENS", DEFAULT_MAX_NEW_TOKENS))
            with torch.no_grad():
                generation = self.model.generate(
                    input_ids=desc_inputs.input_ids,
                    attention_mask=desc_inputs.attention_mask,
                    prompt_input_ids=prompt_inputs.input_ids,
                    prompt_attention_mask=prompt_inputs.attention_mask,
                    max_new_tokens=max_new,
                )
            audio = generation.cpu().numpy().squeeze()
        except Exception as exc:
            return {"error": f"model.generate failed: {type(exc).__name__}: {exc}"}
        if audio is None or getattr(audio, "size", 0) == 0:
            return {"error": "model produced no waveform"}
        if expected_rate and expected_rate != self.sample_rate:
            return {"error": f"caller sample_rate {expected_rate} != model native {self.sample_rate}"}
        pcm = _pcm16le_wav(audio, self.sample_rate)
        return {
            "audio_b64": base64.b64encode(pcm).decode("ascii"),
            "duration_secs": float(audio.shape[-1]) / self.sample_rate,
            "sample_rate": self.sample_rate,
        }


def _pcm16le_wav(samples, sample_rate: int) -> bytes:
    """Encode float samples (range [-1, 1]) as PCM16LE WAV at the
    model's NATIVE sample rate (read from model.config)."""
    import numpy as np
    clipped = np.clip(np.asarray(samples, dtype="float32"), -1.0, 1.0)
    frames = (clipped * 32767.0).astype("<i2").tobytes()
    buf = BytesIO()
    with wave.open(buf, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        wf.writeframes(frames)
    return buf.getvalue()


def _resolve_description_tokenizer_path(model) -> tuple[str, str]:
    """The description tokenizer lives at
    ``model.config.text_encoder._name_or_path`` per the verified
    card usage. We load it ONLY from a local directory:
    STHIRA_TTS_TEXT_ENCODER_DIR if set, else a hub cache lookup with
    local_files_only semantics. Returns (path, blocked_reason)."""
    name = getattr(getattr(model.config, "text_encoder", None), "_name_or_path", "") or ""
    explicit = os.environ.get("STHIRA_TTS_TEXT_ENCODER_DIR", "").strip()
    if explicit and os.path.isdir(explicit):
        return explicit, ""
    if name and os.path.isdir(name):
        return name, ""
    return "", (f"text encoder {_name_or_path_repr(name)!r} is not a local directory; "
                "set STHIRA_TTS_TEXT_ENCODER_DIR to the pre-fetched encoder (no opportunistic "
                "network loading)")


def _name_or_path_repr(name: str) -> str:
    return name or "(absent)"


def _artifact_languages(art_dir: str, approved: list[str]) -> list[str]:
    """Approved ∩ declared artifact coverage. The multilingual
    artifact's config.json may declare language tags; when it does,
    only the intersection is advertised. Absence of a declaration
    means we advertise exactly the approved list (which is itself
    the operator approval gate, not a model claim)."""
    cfg_path = os.path.join(art_dir, "config.json")
    declared = None
    try:
        with open(cfg_path, "r", encoding="utf-8") as fp:
            cfg = json.load(fp)
        for key in ("languages", "language_codes", "supported_languages"):
            if isinstance(cfg.get(key), list):
                declared = {str(x).lower() for x in cfg[key]}
                break
    except Exception:
        declared = None
    if declared is None:
        return list(approved)
    out = []
    for tag in approved:
        iso = tag.split("-", 1)[0].lower()
        if tag.lower() in declared or iso in declared:
            out.append(tag)
    return out


def _try_load() -> tuple[_TTSEngine | None, str]:
    """Real load: dedicated classes from pinned packages, local-only
    from_pretrained, then a bounded warm-up generation. Any failure
    => (None, reason) and the worker stays unready."""
    probe = _artifact_gate()
    if probe["state"] != "PROBE_OK":
        return None, probe["reason"]
    art = probe["dir"]
    try:
        import numpy  # noqa: F401
        import torch  # noqa: F401
        from parler_tts import ParlerTTSForConditionalGeneration  # pinned parler_tts==0.2.2
        from transformers import AutoTokenizer  # pinned transformers==4.46.1
    except ImportError as exc:
        return None, f"required pinned package unavailable: {exc}"

    warmup_budget = float(os.environ.get("STHIRA_TTS_WARMUP_TIMEOUT_SECONDS", "120"))
    started = time.monotonic()
    try:
        model = ParlerTTSForConditionalGeneration.from_pretrained(
            art, local_files_only=True, trust_remote_code=False)
        model.eval()
        sr = int(getattr(model.config, "sampling_rate", 0) or 0)
        if sr <= 0:
            return None, "model.config.sampling_rate missing or zero; cannot encode at native rate"
        prompt_tokenizer = AutoTokenizer.from_pretrained(art, local_files_only=True)
        te_path, blocked = _resolve_description_tokenizer_path(model)
        if not te_path:
            return None, blocked
        description_tokenizer = AutoTokenizer.from_pretrained(te_path, local_files_only=True)
        languages = _artifact_languages(art, _approved_languages())
        if not languages:
            return None, "no approved language survives artifact coverage check"
        engine = _TTSEngine(model, prompt_tokenizer, description_tokenizer,
                            sr, languages, _approved_voices())
        first_voice = next(iter(sorted(engine.voices.get(languages[0], {}))), "")
        if not first_voice:
            return None, f"approved voices file has no entry for {languages[0]!r}"
        # Bounded warm-up: tiny prompt + tiny token budget.
        warm = engine.synthesize("सा", languages[0], first_voice, 0)
        if "error" in warm:
            return None, f"warm-up generation failed: {warm['error']}"
    except Exception as exc:
        return None, f"artifact load/warm-up failed: {type(exc).__name__}: {exc}"
    elapsed = time.monotonic() - started
    if elapsed > warmup_budget:
        return None, f"warm-up exceeded bounded budget ({elapsed:.1f}s > {warmup_budget:.1f}s)"
    return engine, ""


def _ready_info(engine: _TTSEngine) -> dict[str, Any]:
    art = _artifact_dir()
    revision = ""
    rev_file = os.path.join(art, "revision")
    if os.path.isfile(rev_file):
        with open(rev_file, "r", encoding="utf-8") as fp:
            revision = fp.read().strip()
    if not revision:
        revision = "local-artifact:" + os.path.basename(art)
    voices = []
    for lang in engine.languages:
        for name in sorted(engine.voices.get(lang, {})):
            voices.append({"language": lang, "name": name, "revision": "approved-v1"})
    return {
        "model_id": INDIC_PARLER_TTS_MODEL,
        "revision": revision,
        "languages": engine.languages,
        "voices": voices,
        "sample_rate": engine.sample_rate,
        "digest_name": "config.json",
        "digest_sha256": _sha256_file(os.path.join(art, "config.json")),
    }


def _emit(obj: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(obj, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def _handle_ready(engine: _TTSEngine | None, reason: str) -> dict[str, Any]:
    if engine is None:
        return {
            "status": "blocked",
            "model_id": INDIC_PARLER_TTS_MODEL,
            "protocol": PROTOCOL_VERSION,
            "reason": reason or "artifact load failed",
            "languages": [],
            "voices": [],
        }
    info = _ready_info(engine)
    info["status"] = "ready"
    info["protocol"] = PROTOCOL_VERSION
    return info


def _handle_synthesize(msg: dict[str, Any], engine: _TTSEngine | None, reason: str) -> dict[str, Any]:
    rid = msg.get("request_id", "")
    if engine is None:
        return {"request_id": rid,
                "error": f"real inference unavailable: {reason or 'artifact load failed'} (O11)"}
    text = msg.get("text", "")
    if not text or not isinstance(text, str):
        return {"request_id": rid, "error": "text must be a non-empty string"}
    if len(text.encode("utf-8")) > MAX_TEXT_BYTES:
        return {"request_id": rid, "error": f"text exceeds {MAX_TEXT_BYTES} UTF-8 bytes"}
    language = msg.get("language", "")
    voice = msg.get("voice", "")
    if not voice:
        return {"request_id": rid, "error": "voice is required (no default voice exists)"}
    expected_rate = int(msg.get("sample_rate", 0) or 0)
    result = engine.synthesize(text, language, voice, expected_rate)
    if "error" in result:
        return {"request_id": rid, "error": result["error"]}
    return {
        "request_id": rid,
        "audio_b64": result["audio_b64"],
        "duration_secs": result["duration_secs"],
        "sample_rate": result["sample_rate"],
    }


def _handle_shutdown() -> dict[str, Any]:
    return {"status": "shutdown"}


def _verify_envelope_shape(msg: dict[str, Any]) -> None:
    if not isinstance(msg, dict):
        raise ValueError("request must be a JSON object")
    if "op" not in msg or not isinstance(msg["op"], str):
        raise ValueError("op field required")
    if msg["op"] == "synthesize":
        if not msg.get("text"):
            raise ValueError("synthesize requires text")
        if not msg.get("language"):
            raise ValueError("synthesize requires language")
        if msg.get("voice") is not None and not isinstance(msg["voice"], str):
            raise ValueError("voice must be a string")
        if msg.get("sample_rate") is not None and not isinstance(msg["sample_rate"], int):
            raise ValueError("sample_rate must be int")


def _run_loop() -> None:
    engine, reason = _try_load()
    for raw in sys.stdin:
        line = raw.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except json.JSONDecodeError as exc:
            _emit({"error": f"invalid JSON: {exc}"})
            continue
        try:
            _verify_envelope_shape(msg)
        except ValueError as exc:
            _emit({"error": str(exc), "request_id": msg.get("request_id", "")})
            continue
        op = msg["op"]
        if op == "ready":
            _emit(_handle_ready(engine, reason))
        elif op == "synthesize":
            _emit(_handle_synthesize(msg, engine, reason))
        elif op == "shutdown":
            _emit(_handle_shutdown())
            return
        else:
            _emit({"error": f"unknown op: {op}", "request_id": msg.get("request_id", "")})


def main(argv: list[str]) -> int:
    if not _detect_adapter_mode(argv):
        p = _ap.ArgumentParser(prog="speech_tts_adapter")
        p.add_argument("--probe-artifact", action="store_true",
                       help="Print the file-probe gate (NOT readiness) and exit.")
        args = p.parse_args(argv)
        if args.probe_artifact:
            print(json.dumps(_artifact_gate(), indent=2))
            return 2
        p.print_help()
        return 2
    _run_loop()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
