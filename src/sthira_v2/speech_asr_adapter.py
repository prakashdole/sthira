"""JSONL adapter for IndicConformer-600M-Multi ASR.

Protocol (line-delimited JSON over stdin/stdout) is unchanged from
``backend/internal/asrworker/runtime_adapter.go``:

    {"op":"ready"}          -> {"status":"ready"|"blocked", ...}
    {"op":"transcribe",
     "request_id":...}      -> {"request_id":..., "text":..., "confidence":null}
    {"op":"shutdown"}       -> {"status":"shutdown"}

Selected artifact (fixed, approved in plan/decisions.md D59 line):
``ai4bharat/indic-conformer-600m-multilingual`` — multilingual
Conformer hybrid CTC/RNN-T, MIT license, ONNX export layout. The
authoritative model API is documented on the model card:

    model = AutoModel.from_pretrained(repo, trust_remote_code=True)
    text = model(wav, "hi", "ctc")        # returns str only

The returned transcript carries NO confidence score; this adapter
therefore always reports ``confidence: null`` (the Go contract's
"unknown"). It never fabricates a score and never emits a stub
transcript.

Trust model (verified 2026-09-21 against primary sources):

* The repo is gated; only the README/model card was publicly
  retrievable, so the exact per-file inventory of the *multilingual*
  artifact could not be downloaded and hashed here. The ONNX
  execution layout (``assets/preprocessor.ts``, ``assets/encoder.onnx``,
  ``assets/ctc_decoder.onnx``, ``assets/vocab.json``,
  ``assets/language_masks.json``, ``BLANK_ID``/``SOS`` config keys,
  session IO names) is taken from the AI4Bharat ONNX reference
  implementation shipped in the sibling repository
  ``ai4bharat/bhili-asr-conformer-600m-onnx`` (same export tooling,
  MIT), retrieved 2026-09-21. Because that layout may drift from the
  gated multilingual artifact, the loader **discovers** the actual
  artifact: sessions are opened by path, their declared IO names are
  read back from ``onnxruntime``, and any mismatch surfaces as a
  specific BLOCKED reason. Nothing is assumed silently.
* ``trust_remote_code`` model-dir Python is **never executed** by
  this adapter. We drive the ONNX/TorchScript assets directly with
  the pinned local packages (onnxruntime 1.20.1, torch 2.14.0 —
  requirements-voice.txt).
* ``snapshot_download`` is never called. The only load path is a
  pre-populated local directory (``STHIRA_ASR_ARTIFACT_DIR``). A
  missing/partial/unverifiable artifact keeps the worker unready.

READY semantics: the adapter emits ``ready`` ONLY after successful
session creation AND a bounded warm-up forward (tiny 16 kHz buffer
through preprocessor -> encoder -> ctc_decoder). Import errors,
malformed weights, IO-name drift, or a warm-up exception keep it
BLOCKED. Advertised languages are the intersection of the
deployment-approved list (``STHIRA_ASR_APPROVED_LANGUAGES``, default
the two app-enabled locales "hi-IN,ml-IN" per plan/source-register.md)
with the artifact's actual ``vocab``/``language_masks`` keys — model
support never implies government approval, and approval without
artifact support never advertises either.

RNN-T decoding is NOT implemented here (the greedy CTC path is the
verified minimal decoder); ``rnnt`` capability is not advertised.
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
from typing import Any, Callable

INDIC_CONFORMER_MODEL = "ai4bharat/indic-conformer-600m-multilingual"
TARGET_SAMPLE_RATE = 16000
PROTOCOL_VERSION = "asr-adapter-1"
DEFAULT_APPROVED_LANGUAGES = "hi-IN,ml-IN"

# Minimal artifact layout derived from the AI4Bharat ONNX reference
# implementation (see module docstring). Paths are relative to the
# artifact directory.
REQUIRED_ASSETS = (
    "assets/preprocessor.ts",
    "assets/encoder.onnx",
    "assets/ctc_decoder.onnx",
    "assets/vocab.json",
    "assets/language_masks.json",
    "config.json",
)

# Expected session IO names from the reference export. Discovery
# compares these against the actual artifact and fails closed.
ENCODER_IO = {"in": ("audio_signal", "length"), "out": ("outputs", "encoded_lengths")}
CTC_IO = {"in": ("encoder_output",), "out": ("logprobs",)}


def _detect_adapter_mode(argv: list[str]) -> bool:
    return "--adapter-mode" in argv


def _artifact_dir() -> str:
    explicit = os.environ.get("STHIRA_ASR_ARTIFACT_DIR", "").strip()
    if explicit:
        return explicit
    cache = os.path.expanduser(
        "~/.cache/sthira/asr/indic-conformer-600m-multilingual"
    )
    return cache if os.path.isdir(cache) else ""


def _approved_languages() -> list[str]:
    raw = os.environ.get("STHIRA_ASR_APPROVED_LANGUAGES", DEFAULT_APPROVED_LANGUAGES)
    return [tok.strip() for tok in raw.split(",") if tok.strip()]


def bcp47_to_iso(tag: str) -> str:
    """hi-IN -> hi, ml-IN -> ml. Anything without a region keeps
    its bare form; the model's language keys are ISO-639-1 codes."""
    return tag.split("-", 1)[0].lower()


def _sha256_file(path: str) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as fp:
        for chunk in iter(lambda: fp.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def _artifact_gate() -> dict[str, Any]:
    """File-presence probe used ONLY to produce an honest reason
    string. It is not readiness; readiness additionally requires a
    successful session load + warm-up (see _try_load)."""
    art_dir = _artifact_dir()
    if not art_dir:
        return {
            "state": "BLOCKED",
            "reason": "STHIRA_ASR_ARTIFACT_DIR not set and no local cache found (O03)",
            "dir": "",
        }
    missing = [f for f in REQUIRED_ASSETS
               if not os.path.isfile(os.path.join(art_dir, f))]
    if missing:
        return {
            "state": "BLOCKED",
            "reason": f"artifact directory {art_dir!r} incomplete; missing: {', '.join(missing)}",
            "dir": art_dir,
        }
    return {"state": "PROBE_OK", "reason": "", "dir": art_dir}


class _ASREngine:
    """A fully loaded, warm-checked CTC decode path.

    Constructed only from real sessions; every field provenance is
    from the artifact itself. Never instantiated by fakes except in
    tests that inject their own onnxruntime/torch modules.
    """

    def __init__(self, preprocessor, encoder, ctc_decoder, vocab, masks,
                 blank_id: int, frame_duration_ms: float, approved: list[str]):
        self.preprocessor = preprocessor
        self.encoder = encoder
        self.ctc_decoder = ctc_decoder
        self.vocab = vocab
        self.masks = masks
        self.blank_id = blank_id
        self.frame_duration_ms = frame_duration_ms
        # approved ∩ artifact languages (both vocab and mask present)
        self.iso_to_bcp47: dict[str, str] = {}
        for tag in approved:
            iso = bcp47_to_iso(tag)
            if iso in self.vocab and iso in self.masks:
                self.iso_to_bcp47[iso] = tag
        self.languages = sorted(self.iso_to_bcp47.values())

    def decode(self, samples: list[float], sample_rate: int, lang_bcp47: str) -> dict[str, Any]:
        import numpy as np
        iso = next((k for k, v in self.iso_to_bcp47.items() if v == lang_bcp47), None)
        if iso is None:
            return {"error": f"language {lang_bcp47!r} not in loaded+approved {self.languages}"}
        if sample_rate != TARGET_SAMPLE_RATE:
            return {"error": f"sample_rate {sample_rate} not supported; adapter requires {TARGET_SAMPLE_RATE} Hz"}
        import torch
        wav = torch.from_numpy(np.asarray(samples, dtype="float32")).reshape(1, -1)
        audio_signal, length = self.preprocessor(input_signal=wav, length=torch.tensor([wav.shape[-1]]))
        outputs, encoded_lengths = self.encoder.run(
            list(ENCODER_IO["out"]),
            {k: v for k, v in zip(ENCODER_IO["in"], (audio_signal.cpu().numpy(), length.cpu().numpy()))},
        )
        (logprobs,) = self.ctc_decoder.run(list(CTC_IO["out"]), {"encoder_output": outputs})
        import torch as _t
        lp = _t.from_numpy(logprobs)[:, :, self.masks[iso]]
        lp = _t.log_softmax(lp, dim=-1)
        indices = _t.argmax(lp[0], dim=-1)
        collapsed = _t.unique_consecutive(indices, dim=-1)
        table = self.vocab[iso]

        def _tok(idx: int) -> str:
            # JSON objects have string keys; the reference decoder
            # indexes by int. Accept either layout without assuming
            # which the (gated) artifact ships.
            if idx in table:
                return str(table[idx])
            return str(table[str(idx)])

        text = "".join(_tok(int(x)) for x in collapsed if int(x) != self.blank_id)
        text = text.replace("▁", " ").strip()
        # The model returns no score; confidence stays unknown.
        return {"text": text, "confidence": None}


def _discover_io(session, expected_in: tuple[str, ...], expected_out: tuple[str, ...]) -> None:
    """Fail closed unless the session's declared IO names match the
    reference export exactly. Raises ValueError with specifics."""
    ins = tuple(i.name for i in session.get_inputs())
    outs = tuple(o.name for o in session.get_outputs())
    if ins != tuple(expected_in) or outs != tuple(expected_out):
        raise ValueError(
            f"unexpected session IO: got in={ins} out={outs}, "
            f"want in={tuple(expected_in)} out={tuple(expected_out)}"
        )


def _try_load() -> tuple[_ASREngine | None, str]:
    """Attempt the real load. Returns (engine, reason_if_blocked).

    Steps, all bounded and fail-closed: file probe -> imports ->
    session creation with IO discovery -> vocab/mask/config parse
    -> warm-up forward. Any failure returns (None, reason) and the
    worker stays unready.
    """
    probe = _artifact_gate()
    if probe["state"] != "PROBE_OK":
        return None, probe["reason"]
    art = probe["dir"]
    try:
        import numpy  # noqa: F401  (provenance check: installed, pinned)
        import torch
        import onnxruntime as ort
    except ImportError as exc:
        return None, f"required pinned package unavailable: {exc}"

    warmup_budget = float(os.environ.get("STHIRA_ASR_WARMUP_TIMEOUT_SECONDS", "30"))
    started = time.monotonic()
    try:
        with open(os.path.join(art, "config.json"), "r", encoding="utf-8") as fp:
            cfg = json.load(fp)
        with open(os.path.join(art, "assets/vocab.json"), "r", encoding="utf-8") as fp:
            vocab = json.load(fp)
        with open(os.path.join(art, "assets/language_masks.json"), "r", encoding="utf-8") as fp:
            masks = json.load(fp)
        if not isinstance(vocab, dict) or not vocab:
            raise ValueError("vocab.json is empty or malformed")
        if not isinstance(masks, dict) or not masks:
            raise ValueError("language_masks.json is empty or malformed")
        blank_id = int(cfg.get("BLANK_ID", 256))
        frame_ms = float(cfg.get("FRAME_DURATION_MS", 0.08))

        preprocessor = torch.jit.load(
            os.path.join(art, "assets/preprocessor.ts"), map_location="cpu")
        encoder = ort.InferenceSession(
            os.path.join(art, "assets/encoder.onnx"), providers=["CPUExecutionProvider"])
        _discover_io(encoder, ENCODER_IO["in"], ENCODER_IO["out"])
        ctc_decoder = ort.InferenceSession(
            os.path.join(art, "assets/ctc_decoder.onnx"), providers=["CPUExecutionProvider"])
        _discover_io(ctc_decoder, CTC_IO["in"], CTC_IO["out"])

        engine = _ASREngine(preprocessor, encoder, ctc_decoder, vocab, masks,
                            blank_id, frame_ms, _approved_languages())
        if not engine.languages:
            return None, ("no approved language is present in the artifact "
                          f"(approved={_approved_languages()}, artifact vocab keys={sorted(vocab)[:8]}…)")
        # Bounded warm-up: 0.16 s of zeros through the full graph.
        import numpy as np
        warm = np.zeros(int(TARGET_SAMPLE_RATE * 0.16), dtype="float32").tolist()
        engine.decode(warm, TARGET_SAMPLE_RATE, engine.languages[0])
    except Exception as exc:  # malformed weights, IO drift, torch/ONNX errors
        return None, f"artifact load/warm-up failed: {type(exc).__name__}: {exc}"
    elapsed = time.monotonic() - started
    if elapsed > warmup_budget:
        return None, f"warm-up exceeded bounded budget ({elapsed:.1f}s > {warmup_budget:.1f}s)"
    return engine, ""


def _ready_info(engine: _ASREngine) -> dict[str, Any]:
    art = _artifact_dir()
    revision = ""
    rev_file = os.path.join(art, "revision")
    if os.path.isfile(rev_file):
        with open(rev_file, "r", encoding="utf-8") as fp:
            revision = fp.read().strip()
    if not revision:
        revision = "local-artifact:" + os.path.basename(art)
    return {
        "model_id": INDIC_CONFORMER_MODEL,
        "revision": revision,
        "languages": engine.languages,
        "digest_name": "config.json",
        "digest_sha256": _sha256_file(os.path.join(art, "config.json")),
        "decoding": "ctc-greedy",
    }


def _emit(obj: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(obj, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def _handle_ready(engine: _ASREngine | None, reason: str) -> dict[str, Any]:
    if engine is None:
        return {
            "status": "blocked",
            "model_id": INDIC_CONFORMER_MODEL,
            "protocol": PROTOCOL_VERSION,
            "reason": reason or "artifact load failed",
            "languages": [],
        }
    info = _ready_info(engine)
    info["status"] = "ready"
    info["protocol"] = PROTOCOL_VERSION
    return info


def _handle_transcribe(msg: dict[str, Any], engine: _ASREngine | None, reason: str) -> dict[str, Any]:
    rid = msg.get("request_id", "")
    if engine is None:
        return {"request_id": rid,
                "error": f"real inference unavailable: {reason or 'artifact load failed'} (O03)"}
    try:
        raw = base64.b64decode(msg.get("samples_b64", "").encode("ascii"))
    except Exception as exc:
        return {"request_id": rid, "error": f"samples_b64 decode: {exc}"}
    if len(raw) % 4 != 0:
        return {"request_id": rid, "error": "samples_b64 byte length not multiple of 4"}
    n = len(raw) // 4
    samples = list(struct.unpack("<%df" % n, raw))
    if n > 320000:  # 20 s @ 16 kHz, the Go bounded-input ceiling
        return {"request_id": rid, "error": "decoded samples exceed limit"}
    lang = msg.get("language") or ""
    result = engine.decode(samples, int(msg.get("sample_rate", 0) or 0), lang)
    if "error" in result:
        return {"request_id": rid, "error": result["error"]}
    return {
        "request_id": rid,
        "text": result["text"],
        "confidence": result["confidence"],  # always None: model emits no score
        "alternatives": [],
        "duration_secs": float(msg.get("duration_secs", n / TARGET_SAMPLE_RATE)),
    }


def _handle_shutdown() -> dict[str, Any]:
    return {"status": "shutdown"}


def _verify_envelope_shape(msg: dict[str, Any]) -> None:
    if not isinstance(msg, dict):
        raise ValueError("request must be a JSON object")
    if "op" not in msg or not isinstance(msg["op"], str):
        raise ValueError("op field required")
    if msg["op"] == "transcribe":
        if not msg.get("request_id"):
            raise ValueError("transcribe requires request_id")
        if msg.get("samples_b64") is None:
            raise ValueError("transcribe requires samples_b64")
        if not isinstance(msg.get("sample_rate"), int):
            raise ValueError("transcribe requires sample_rate int")


def _run_loop() -> None:
    """Main JSONL loop.

    Load is attempted ONCE at startup, BEFORE the first ready
    response, so ``ready`` reports the actual loaded+warm state —
    not a file-presence guess. A failing load keeps the adapter
    alive to report BLOCKED per call (the Go dispatcher requires a
    well-formed response), but nothing beyond refusing inference is
    possible.
    """
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
        elif op == "transcribe":
            _emit(_handle_transcribe(msg, engine, reason))
        elif op == "shutdown":
            _emit(_handle_shutdown())
            return
        else:
            _emit({"error": f"unknown op: {op}", "request_id": msg.get("request_id", "")})


def main(argv: list[str]) -> int:
    if not _detect_adapter_mode(argv):
        p = _ap.ArgumentParser(prog="speech_asr_adapter")
        p.add_argument("--probe-artifact", action="store_true",
                       help="Print the file-probe gate (NOT readiness) and exit.")
        args = p.parse_args(argv)
        if args.probe_artifact:
            print(json.dumps(_artifact_gate(), indent=2))
            return 0
        p.print_help()
        return 2
    _run_loop()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
