"""Real-entry-point tests for the v2 ASR/TTS adapters with injected
fake libraries.

The adapters are spawned as subprocesses with PYTHONPATH pointing at
generated fake packages (torch, onnxruntime, parler_tts,
transformers). This exercises the ACTUAL adapter code paths —
discovery, IO validation, load ordering, warm-up, real invocation,
result decoding, WAV encoding — without weights, GPU, or network.
Per the worker prompt: these are PROTOCOL/ordering evidence, not
model-quality evidence. The fake call trace file proves
load -> warm -> ready ordering because the adapter emits "ready"
only after _try_load (which includes the warm-up) has returned.

Run: PYTHONPATH=src pytest tests/test_v2_real_adapters.py
"""

from __future__ import annotations

import base64
import json
import os
import struct
import subprocess
import sys
import wave
from io import BytesIO

import pytest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
SRC_ROOT = os.path.join(REPO_ROOT, "src")


FAKE_TORCH = '''"""Minimal fake torch over numpy, tracing every factory call."""
import numpy as _np
import os as _os
from contextlib import contextmanager as _cm

TRACE = _os.environ.get("STHIRA_FAKE_TRACE", "")

def _log(name):
    if TRACE:
        with open(TRACE, "a") as f:
            f.write(name + "\\n")

class Tensor:
    def __init__(self, a):
        self.a = _np.asarray(a)
    def __getitem__(self, k):
        return Tensor(self.a[k])
    def __iter__(self):
        return iter(self.a.tolist())
    @property
    def shape(self):
        return self.a.shape
    def cpu(self):
        return self
    def numpy(self):
        return self.a
    def reshape(self, *s):
        return Tensor(self.a.reshape(*s))
    def log_softmax(self, dim=-1):
        return log_softmax(self, dim=dim)
    def argmax(self, dim=-1):
        return argmax(self, dim=dim)

def from_numpy(a):
    return Tensor(a)

def tensor(a, dtype=None):
    return Tensor(_np.asarray(a))

def log_softmax(t, dim=-1):
    x = _np.asarray(t.a if isinstance(t, Tensor) else t, dtype="float64")
    m = x.max(axis=dim, keepdims=True)
    e = _np.exp(x - m)
    return Tensor(x - m - _np.log(e.sum(axis=dim, keepdims=True)))

def argmax(t, dim=-1):
    x = t.a if isinstance(t, Tensor) else _np.asarray(t)
    return Tensor(_np.argmax(x, axis=dim))

def unique_consecutive(t, dim=-1):
    x = t.a if isinstance(t, Tensor) else _np.asarray(t)
    out = []
    prev = None
    for v in x.tolist():
        if v != prev:
            out.append(v)
            prev = v
    return Tensor(_np.array(out))

@_cm
def no_grad():
    _log("no_grad")
    yield

class _JIT:
    def load(self, path, map_location=None):
        _log("jit.load:" + _os.path.basename(path))
        def preprocessor(input_signal=None, length=None):
            _log("preprocessor")
            return Tensor([[0.0]]), Tensor([1])
        return preprocessor

jit = _JIT()
'''

FAKE_ORT = '''"""Fake onnxruntime: InferenceSession honors the STHIRA_FAKE_* env
to either succeed (with scripted outputs) or fail, and traces."""
import json as _json
import numpy as _np
import os as _os

TRACE = _os.environ.get("STHIRA_FAKE_TRACE", "")
CTC_FRAMES = _json.loads(_os.environ.get("STHIRA_FAKE_CTC_ARGMAX", "[]"))
IO_BAD = _os.environ.get("STHIRA_FAKE_BAD_IO") == "1"

class _IO:
    def __init__(self, name):
        self.name = name

class InferenceSession:
    def __init__(self, path, providers=None):
        _os.environ["STHIRA_FAKE_SEEN_" + _os.path.basename(path).replace(".", "_")] = "1"
        self.path = path
    def get_inputs(self):
        if _os.path.basename(self.path) == "encoder.onnx":
            return [_IO("bogus_in")] if IO_BAD else [_IO("audio_signal"), _IO("length")]
        return [_IO("encoder_output")]
    def get_outputs(self):
        if _os.path.basename(self.path) == "encoder.onnx":
            return [_IO("outputs"), _IO("encoded_lengths")]
        return [_IO("logprobs")]
    def run(self, output_names, inputs):
        name = _os.path.basename(self.path)
        if TRACE:
            with open(TRACE, "a") as f:
                f.write("run:" + name + "\\n")
        if name == "encoder.onnx":
            return (_np.zeros((1, 2, 2), dtype="float32"), _np.array([1]))
        # ctc_decoder: one-hot log-probs along the scripted argmax
        # path (empty script -> one blank-ish frame, enough to warm).
        vocab_size = int(_os.environ.get("STHIRA_FAKE_VOCAB_SIZE", "5"))
        path = CTC_FRAMES if CTC_FRAMES else [0]
        frames = [[float(10.0 if c == t else -10.0) for c in range(vocab_size)]
                  for t in path]
        arr = _np.array(frames, dtype="float32")[_np.newaxis, :, :]
        return (arr,)
'''


def _make_asr_artifact(tmp_path):
    art = tmp_path / "conformer"
    (art / "assets").mkdir(parents=True)
    (art / "config.json").write_text(json.dumps(
        {"BLANK_ID": 4, "FRAME_DURATION_MS": 0.08, "SOS": 5632}))
    vocab = {"hi": {str(i): ch for i, ch in enumerate("abcd<blk>")}}
    (art / "assets" / "vocab.json").write_text(json.dumps(vocab))
    masks = {"hi": [0, 1, 2, 3, 4]}
    (art / "assets" / "language_masks.json").write_text(json.dumps(masks))
    for f in ("encoder.onnx", "ctc_decoder.onnx", "preprocessor.ts"):
        (art / "assets" / f).write_bytes(b"fake-artifact")
    return art


def _run(module, requests, tmp_path, fake_dir, env_extra=None):
    env = os.environ.copy()
    env["PYTHONPATH"] = os.pathsep.join(
        [str(fake_dir), SRC_ROOT] + ([env.get("PYTHONPATH", "")] if env.get("PYTHONPATH") else []))
    env.pop("STHIRA_ASR_ARTIFACT_DIR", None)
    env.pop("STHIRA_TTS_ARTIFACT_DIR", None)
    trace = str(tmp_path / "trace.log")
    env["STHIRA_FAKE_TRACE"] = trace
    if env_extra:
        env.update(env_extra)
    payload = "".join(json.dumps(r, separators=(",", ":")) + "\n" for r in requests)
    payload += json.dumps({"op": "shutdown"}) + "\n"
    proc = subprocess.run(
        [sys.executable, "-u", "-m", module, "--adapter-mode"],
        input=payload, capture_output=True, text=True, env=env,
        cwd=REPO_ROOT, timeout=60)
    responses = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if line:
            try:
                responses.append(json.loads(line))
            except json.JSONDecodeError:
                pass
    return responses, trace, proc


@pytest.fixture()
def fake_dir(tmp_path):
    d = tmp_path / "fakes"
    d.mkdir()
    (d / "torch.py").write_text(FAKE_TORCH)
    (d / "onnxruntime.py").write_text(FAKE_ORT)
    return d


def _samples_b64(n=8):
    return base64.b64encode(struct.pack("<%df" % n, *([0.01] * n))).decode("ascii")


class TestASRRealAdapter:
    def test_load_warm_ready_order_and_intersection(self, tmp_path, fake_dir):
        art = _make_asr_artifact(tmp_path)
        # Approve hi-IN + a language NOT in the artifact vocab: only
        # hi-IN may be advertised (approved ∩ artifact).
        res, trace, proc = _run(
            "sthira_v2.speech_asr_adapter",
            [{"op": "ready"}],
            tmp_path, fake_dir,
            {"STHIRA_ASR_ARTIFACT_DIR": str(art),
             "STHIRA_ASR_APPROVED_LANGUAGES": "hi-IN,ta-IN"})
        ready = res[0]
        assert ready["status"] == "ready", ready
        assert ready["languages"] == ["hi-IN"]
        assert ready["revision"].startswith("local-artifact:")
        assert len(ready["digest_sha256"]) == 64
        lines = open(trace).read().splitlines() if os.path.exists(trace) else []
        # Ordering: sessions created + warm-up runs executed BEFORE
        # the ready line is emitted (adapter returns only then).
        assert "jit.load:preprocessor.ts" in lines
        assert lines.count("run:encoder.onnx") >= 1
        assert lines.count("run:ctc_decoder.onnx") >= 1
        assert lines.index("run:ctc_decoder.onnx") < len(lines)

    def test_transcribe_real_decode_path_no_confidence(self, tmp_path, fake_dir):
        art = _make_asr_artifact(tmp_path)
        # argmax path: a, blank(4), b, b, c, d, d -> collapse -> a b? no:
        # [0,4,1,2,3] minus blank -> a,b,c,d -> "abcd" (no spaces)
        res, _, proc = _run(
            "sthira_v2.speech_asr_adapter",
            [{"op": "transcribe", "request_id": "R-1",
              "samples_b64": _samples_b64(), "sample_rate": 16000,
              "language": "hi-IN"}],
            tmp_path, fake_dir,
            {"STHIRA_ASR_ARTIFACT_DIR": str(art),
             "STHIRA_FAKE_CTC_ARGMAX": "[0,4,1,1,2,3,3]"})
        tr = res[0]
        assert tr["request_id"] == "R-1"
        assert tr["text"] == "abcd", tr
        assert tr["confidence"] is None  # never fabricated
        assert "unverified" not in json.dumps(tr)  # stub string is gone

    def test_io_drift_fails_closed(self, tmp_path, fake_dir):
        art = _make_asr_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_asr_adapter", [{"op": "ready"}],
            tmp_path, fake_dir,
            {"STHIRA_ASR_ARTIFACT_DIR": str(art), "STHIRA_FAKE_BAD_IO": "1"})
        ready = res[0]
        assert ready["status"] == "blocked"
        assert "unexpected session IO" in ready["reason"]

    def test_import_failure_stays_unready(self, tmp_path, fake_dir):
        art = _make_asr_artifact(tmp_path)
        (fake_dir / "onnxruntime.py").write_text("raise ImportError('no ort')\n")
        res, _, _ = _run(
            "sthira_v2.speech_asr_adapter", [{"op": "ready"}],
            tmp_path, fake_dir,
            {"STHIRA_ASR_ARTIFACT_DIR": str(art)})
        ready = res[0]
        assert ready["status"] == "blocked"
        assert "unavailable" in ready["reason"] or "ImportError" in ready["reason"]

    def test_wrong_rate_rejected(self, tmp_path, fake_dir):
        art = _make_asr_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_asr_adapter",
            [{"op": "transcribe", "request_id": "R-2",
              "samples_b64": _samples_b64(), "sample_rate": 8000,
              "language": "hi-IN"}],
            tmp_path, fake_dir,
            {"STHIRA_ASR_ARTIFACT_DIR": str(art)})
        assert "16000" in res[0]["error"]

    def test_unapproved_language_refused(self, tmp_path, fake_dir):
        art = _make_asr_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_asr_adapter",
            [{"op": "transcribe", "request_id": "R-3",
              "samples_b64": _samples_b64(), "sample_rate": 16000,
              "language": "ml-IN"}],
            tmp_path, fake_dir,
            {"STHIRA_ASR_ARTIFACT_DIR": str(art),
             "STHIRA_ASR_APPROVED_LANGUAGES": "hi-IN"})
        assert "not in loaded+approved" in res[0]["error"]


# ---------------- TTS fakes ----------------

FAKE_TTS_MODEL = '''"""Fake parler_tts + transformers exposing the verified Indic
Parler API surface: dual tokenizers, generate(input_ids=...,
prompt_input_ids=...), config.sampling_rate."""
import numpy as _np
import os as _os

TRACE = _os.environ.get("STHIRA_FAKE_TRACE", "")

def _log(name):
    if TRACE:
        with open(TRACE, "a") as f:
            f.write(name + "\\n")

class _Tensor:
    def __init__(self, a): self.a = _np.asarray(a)
    def cpu(self): return self
    def numpy(self): return self.a
    def squeeze(self): return self.a.squeeze()

class _Cfg:
    def __init__(self, sr):
        self.sampling_rate = sr
        class _TE:
            _name_or_path = "local-text-encoder"
        self.text_encoder = _TE()

class _Batch:
    def __init__(self, n):
        self.input_ids = _np.arange(n).reshape(1, n)
        self.attention_mask = _np.ones((1, n))

class FakeTokenizer:
    def __init__(self, kind):
        self.kind = kind
    def __call__(self, text, return_tensors=None):
        _log("tokenize:" + self.kind)
        return _Batch(max(1, len(text)))

class ParlerTTSForConditionalGeneration:
    @staticmethod
    def from_pretrained(path, local_files_only=False, trust_remote_code=False):
        _log("load:" + _os.path.basename(str(path)))
        assert local_files_only, "opportunistic network loading is forbidden"
        assert not trust_remote_code
        sr = int(_os.environ.get("STHIRA_FAKE_TTS_SR", "44100"))
        if _os.environ.get("STHIRA_FAKE_TTS_FAIL") == "load":
            raise RuntimeError("malformed weights")
        m = ParlerTTSForConditionalGeneration()
        m.config = _Cfg(sr)
        return m
    def __init__(self): pass
    def eval(self): _log("eval")
    def generate(self, input_ids=None, attention_mask=None,
                 prompt_input_ids=None, prompt_attention_mask=None,
                 max_new_tokens=None):
        _log("generate")
        if _os.environ.get("STHIRA_FAKE_TTS_FAIL") == "generate":
            raise RuntimeError("warm-up boom")
        n = 4410  # 0.1 s @ 44.1 kHz
        return _Tensor(_np.full((1, 1, n), 0.25, dtype="float32") * 0.0 +
                       _np.linspace(-0.5, 0.5, n).reshape(1, 1, n))

class AutoTokenizer:
    @staticmethod
    def from_pretrained(path, local_files_only=False):
        _log("tokenizer:" + _os.path.basename(str(path)))
        assert local_files_only
        return FakeTokenizer(_os.path.basename(str(path)))
'''


def _make_tts_artifact(tmp_path):
    art = tmp_path / "parler"
    art.mkdir()
    (art / "config.json").write_text("{}")
    (art / "model.safetensors").write_bytes(b"fake-weights")
    enc = art / "text_encoder"
    enc.mkdir()
    (enc / "tokenizer_config.json").write_text("{}")
    voices = art / "voices.json"
    voices.write_text(json.dumps(
        {"hi-IN": {"Rohit": "Rohit speaks clearly."},
         "ml-IN": {"Anjali": "Anjali speaks clearly."}}))
    return art, voices


class TestTTSRealAdapter:
    def _env(self, art, voices, extra=None):
        e = {"STHIRA_TTS_ARTIFACT_DIR": str(art),
             "STHIRA_TTS_TEXT_ENCODER_DIR": str(art / "text_encoder"),
             "STHIRA_TTS_VOICES_FILE": str(voices)}
        if extra:
            e.update(extra)
        return e

    def test_load_warm_ready_ordering_and_native_rate(self, tmp_path, fake_dir):
        (fake_dir / "parler_tts.py").write_text(FAKE_TTS_MODEL)
        (fake_dir / "transformers.py").write_text(
            "from parler_tts import AutoTokenizer\n")
        art, voices = _make_tts_artifact(tmp_path)
        res, trace, proc = _run(
            "sthira_v2.speech_tts_adapter", [{"op": "ready"}],
            tmp_path, fake_dir, self._env(art, voices))
        ready = res[0]
        assert ready["status"] == "ready", ready
        assert ready["sample_rate"] == 44100
        assert {"language": "hi-IN", "name": "Rohit",
                "revision": "approved-v1"} in ready["voices"]
        lines = open(trace).read().splitlines()
        assert lines.index("load:parler") < lines.index("generate")
        assert lines.index("generate") < len(lines)  # warm-up before ready

    def test_synthesize_encodes_at_native_rate(self, tmp_path, fake_dir):
        (fake_dir / "parler_tts.py").write_text(FAKE_TTS_MODEL)
        (fake_dir / "transformers.py").write_text(
            "from parler_tts import AutoTokenizer\n")
        art, voices = _make_tts_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_tts_adapter",
            [{"op": "synthesize", "request_id": "S-1", "text": "नमस्ते",
              "language": "hi-IN", "voice": "Rohit", "sample_rate": 44100}],
            tmp_path, fake_dir, self._env(art, voices))
        out = res[0]
        assert out["request_id"] == "S-1"
        wav_bytes = base64.b64decode(out["audio_b64"])
        wf = wave.open(BytesIO(wav_bytes), "rb")
        assert wf.getframerate() == 44100  # native, not 22050
        assert wf.getnchannels() == 1
        assert wf.getsampwidth() == 2
        dur = out["duration_secs"]
        assert abs(dur - 0.1) < 0.01

    def test_wrong_expected_rate_refused(self, tmp_path, fake_dir):
        (fake_dir / "parler_tts.py").write_text(FAKE_TTS_MODEL)
        (fake_dir / "transformers.py").write_text(
            "from parler_tts import AutoTokenizer\n")
        art, voices = _make_tts_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_tts_adapter",
            [{"op": "synthesize", "request_id": "S-2", "text": "hi",
              "language": "hi-IN", "voice": "Rohit", "sample_rate": 22050}],
            tmp_path, fake_dir, self._env(art, voices))
        assert "native" in res[0]["error"]

    def test_no_voices_file_blocks(self, tmp_path, fake_dir):
        (fake_dir / "parler_tts.py").write_text(FAKE_TTS_MODEL)
        (fake_dir / "transformers.py").write_text(
            "from parler_tts import AutoTokenizer\n")
        art, _ = _make_tts_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_tts_adapter", [{"op": "ready"}],
            tmp_path, fake_dir, {"STHIRA_TTS_ARTIFACT_DIR": str(art)})
        ready = res[0]
        assert ready["status"] == "blocked"
        assert "VOICES" in ready["reason"].upper()

    def test_warmup_failure_keeps_unready(self, tmp_path, fake_dir):
        (fake_dir / "parler_tts.py").write_text(FAKE_TTS_MODEL)
        (fake_dir / "transformers.py").write_text(
            "from parler_tts import AutoTokenizer\n")
        art, voices = _make_tts_artifact(tmp_path)
        res, _, _ = _run(
            "sthira_v2.speech_tts_adapter", [{"op": "ready"}],
            tmp_path, fake_dir,
            self._env(art, voices, {"STHIRA_FAKE_TTS_FAIL": "generate"}))
        assert res[0]["status"] == "blocked"
        assert "warm-up" in res[0]["reason"]

    def test_unapproved_voice_and_language_refused(self, tmp_path, fake_dir):
        (fake_dir / "parler_tts.py").write_text(FAKE_TTS_MODEL)
        (fake_dir / "transformers.py").write_text(
            "from parler_tts import AutoTokenizer\n")
        art, voices = _make_tts_artifact(tmp_path)
        env = self._env(art, voices,
                        {"STHIRA_TTS_APPROVED_LANGUAGES": "hi-IN"})
        res, _, _ = _run(
            "sthira_v2.speech_tts_adapter",
            [{"op": "synthesize", "request_id": "S-3", "text": "hi",
              "language": "ml-IN", "voice": "Anjali", "sample_rate": 44100},
             {"op": "synthesize", "request_id": "S-4", "text": "hi",
              "language": "hi-IN", "voice": "Ghost", "sample_rate": 44100}],
            tmp_path, fake_dir, env)
        assert "not in approved+advertised" in res[0]["error"]
        assert "not approved" in res[1]["error"]
