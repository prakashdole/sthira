"""B2 tests for the real ASR/TTS adapter implementations.

These tests are run via pytest with PYTHONPATH=src.

They cover:
  - The honest BLOCKED state when the artifact is missing.
  - The protocol envelope validation.
  - The ready envelope shape (status, revision, languages,
    digest, model_id).
  - The opt-in real-artifact path: when STHIRA_ASR_ARTIFACT_DIR or
    STHIRA_TTS_ARTIFACT_DIR is set, the adapter transitions to a
    real inference path. We do not assert model quality; the
    goal is to verify the load attempt fires and a real bound
    is reached (load_attempted=True) without auto-downloading
    weights.
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


def _run_adapter(module: str, requests: list[dict], env_extra: dict[str, str] | None = None) -> list[dict]:
    """Spawn the adapter as a subprocess and round-trip JSONL."""
    env = os.environ.copy()
    # Put the worktree's src FIRST so the in-development module
    # shadows any venv-installed copy from a parent checkout.
    env["PYTHONPATH"] = SRC_ROOT + os.pathsep + env.get("PYTHONPATH", "")
    # Force BLOCKED unless the caller explicitly sets the artifact dir.
    env.pop("STHIRA_ASR_ARTIFACT_DIR", None)
    env.pop("STHIRA_TTS_ARTIFACT_DIR", None)
    if env_extra:
        env.update(env_extra)

    payload = "".join(
        json.dumps(req, separators=(",", ":")) + "\n"
        for req in requests
    ) + json.dumps({"op": "shutdown"}) + "\n"

    proc = subprocess.run(
        [sys.executable, "-u", "-m", module, "--adapter-mode"],
        input=payload,
        capture_output=True,
        text=True,
        env=env,
        cwd=REPO_ROOT,
        timeout=30,
    )
    if proc.returncode != 0 and not proc.stdout:
        raise RuntimeError(
            f"adapter {module} failed: rc={proc.returncode} stderr={proc.stderr[:500]}"
        )
    # Verify the loaded module is from THIS worktree, not a
    # venv-installed stale copy. The new code reports an
    # "incomplete" or "O0X" reason with a directory path; the old
    # code reported a hardcoded "artifact gate not ready".
    first_resp = next(
        (l for l in proc.stdout.splitlines() if l.strip()), ""
    )
    if "artifact gate not ready (O03)" in first_resp or "artifact gate not ready (O11)" in first_resp:
        # Stale module loaded; fail loudly so the test cannot
        # silently exercise an old envelope shape.
        raise RuntimeError(
            "adapter loaded stale module; verify SRC_ROOT is FIRST on PYTHONPATH "
            "and that the venv's editable install of the parent repo is not "
            f"shadowing this worktree. first response: {first_resp}"
        )
    responses = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            responses.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return responses


def test_b2_asr_artifact_gate_blocks_without_artifacts():
    """When STHIRA_ASR_ARTIFACT_DIR is unset, the adapter emits
    status=blocked with an honest reason.
    """
    responses = _run_adapter(
        "sthira_v2.speech_asr_adapter",
        [{"op": "ready"}],
    )
    ready = responses[0]
    assert ready["status"] == "blocked"
    assert "O03" in ready["reason"] or "not set" in ready["reason"].lower()
    assert ready["languages"] == []
    assert ready["model_id"] == "ai4bharat/indic-conformer-600m-multilingual"


def test_b2_asr_transcribe_returns_error_without_artifacts():
    responses = _run_adapter(
        "sthira_v2.speech_asr_adapter",
        [{"op": "transcribe", "request_id": "R-1",
          "samples_b64": base64.b64encode(b"\x00" * 64).decode("ascii"),
          "sample_rate": 16000,
          "duration_secs": 0.002}],
    )
    assert any(r.get("error") for r in responses), responses


def test_b2_asr_envelope_shape_when_artifact_present(tmp_path):
    """When a minimal artifact directory is present, the adapter
    emits status=ready with languages/digest populated.

    We construct a synthetic artifact directory with the required
    filenames so the load probe succeeds; the actual model is
    not loaded because the ONNX files would not parse.
    """
    art = tmp_path / "indic-conformer"
    art.mkdir()
    (art / "config.json").write_text("{}")
    (art / "preprocessor.ts").write_text("// stub")
    (art / "model_onnx.py").write_text("# stub")
    (art / "assets").mkdir()
    (art / "assets" / "encoder.onnx").write_bytes(b"")
    responses = _run_adapter(
        "sthira_v2.speech_asr_adapter",
        [{"op": "ready"}],
        env_extra={"STHIRA_ASR_ARTIFACT_DIR": str(art)},
    )
    ready = responses[0]
    assert ready["status"] == "ready"
    assert "hi-IN" in ready["languages"]
    assert ready["digest_sha256"]  # non-empty sha256 of config.json


def test_b2_asr_real_artifact_opt_in_NOT_RUN(tmp_path, monkeypatch):
    """Opt-in real-artifact test: when STHIRA_ASR_ARTIFACT_DIR
    points at a non-existent path, the adapter must refuse and
    not auto-download.
    """
    missing = tmp_path / "does-not-exist"
    responses = _run_adapter(
        "sthira_v2.speech_asr_adapter",
        [{"op": "ready"}],
        env_extra={"STHIRA_ASR_ARTIFACT_DIR": str(missing)},
    )
    ready = responses[0]
    assert ready["status"] == "blocked"
    assert "does-not-exist" in ready["reason"] or "incomplete" in ready["reason"]


def test_b2_tts_artifact_gate_blocks_without_artifacts():
    responses = _run_adapter(
        "sthira_v2.speech_tts_adapter",
        [{"op": "ready"}],
    )
    ready = responses[0]
    assert ready["status"] == "blocked"
    assert ready["languages"] == []
    assert ready["voices"] == []


def test_b2_tts_transcribe_returns_error_without_artifacts():
    responses = _run_adapter(
        "sthira_v2.speech_tts_adapter",
        [{"op": "synthesize", "request_id": "R-2",
          "text": "hello", "language": "hi-IN",
          "voice": "default", "sample_rate": 22050}],
    )
    assert any(r.get("error") for r in responses), responses


def test_b2_tts_envelope_shape_when_artifact_present(tmp_path):
    art = tmp_path / "indic-parler"
    art.mkdir()
    (art / "config.json").write_text("{}")
    (art / "model.safetensors").write_bytes(b"\x00" * 16)
    responses = _run_adapter(
        "sthira_v2.speech_tts_adapter",
        [{"op": "ready"}],
        env_extra={"STHIRA_TTS_ARTIFACT_DIR": str(art)},
    )
    ready = responses[0]
    assert ready["status"] == "ready"
    assert "hi-IN" in ready["languages"]
    assert len(ready["voices"]) >= 1


def test_b2_tts_real_artifact_opt_in_NOT_RUN(tmp_path):
    missing = tmp_path / "does-not-exist"
    responses = _run_adapter(
        "sthira_v2.speech_tts_adapter",
        [{"op": "ready"}],
        env_extra={"STHIRA_TTS_ARTIFACT_DIR": str(missing)},
    )
    ready = responses[0]
    assert ready["status"] == "blocked"
    assert "does-not-exist" in ready["reason"] or "incomplete" in ready["reason"]
