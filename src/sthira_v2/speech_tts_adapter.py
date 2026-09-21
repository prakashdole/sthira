"""JSONL adapter for Indic Parler-TTS.

This module implements the line-delimited JSON protocol expected by
backend/internal/ttsworker/runtime_adapter.go. It is invoked as:

    python3 -m sthira_v2.speech_tts_adapter --adapter-mode

The adapter reads JSONL requests from stdin and emits JSONL responses
on stdout. Each request has an "op" field:

    {"op":"ready"}              →  ready envelope with languages/voices/digest
    {"op":"synthesize",         →  {"audio_b64":..., "duration_secs":...}
     "text":..., "language":...}
    {"op":"shutdown"}           →  graceful exit

Honesty gate (BLOCKED_EXTERNAL semantics):

Real speech generation requires the Indic Parler-TTS model weights
and the speaker prompts, which are not present in this revision.
Until authorized artifacts land (per plan/open-decisions.md O11),
the adapter refuses inference on every "synthesize" request and
reports a BLOCKED state.

    {"status":"blocked","reason":"artifact gate not ready"}

The orchestrator surfaces this as ErrRuntimeUnavailable. No canned
or deterministic audio is emitted.

When artifacts become available (post O11 resolution), the
`load_artifact` function is the single seam to add real model loading
code; the protocol code does not change.
"""

from __future__ import annotations

import argparse as _ap
import base64
import hashlib
import json
import sys
from typing import Any


INDIC_PARLER_TTS_MODEL = "ai4bharat/indic-parler-tts"
TARGET_SAMPLE_RATE = 22050
PROTOCOL_VERSION = "tts-adapter-1"


def _detect_adapter_mode(argv: list[str]) -> bool:
    return "--adapter-mode" in argv


def _artifact_gate() -> dict[str, Any]:
    """Honest artifact gate. Returns BLOCKED until weights are installed.

    The single seam for wiring real model loading is here: replace
    the body with the loader that materializes Indic Parler-TTS from
    the pinned local artifact directory. Until then, this is the
    truthful "blocked real inference" state the Stage 6 review
    demanded.
    """
    return {
        "model_id": INDIC_PARLER_TTS_MODEL,
        "revision": "",
        "languages": [],
        "voices": [],
        "digest_name": "",
        "digest_sha256": "",
        "state": "BLOCKED",
        "reason": "real inference adapter: artifact gate not ready (O11)",
    }


def _emit(obj: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(obj, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def _handle_ready() -> dict[str, Any]:
    gate = _artifact_gate()
    return {
        "status": "blocked",
        "model_id": gate["model_id"],
        "protocol": PROTOCOL_VERSION,
        "reason": gate["reason"],
    }


def _handle_synthesize(msg: dict[str, Any]) -> dict[str, Any]:
    return {
        "error": "artifact gate blocked: real inference not wired (O11)",
    }


def _handle_shutdown() -> dict[str, Any]:
    return {"status": "shutdown"}


def _verify_envelope_shape(msg: dict[str, Any]) -> None:
    """Structural check on inbound request envelopes."""
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


def _run_loop() -> None:
    """Main JSONL read/eval loop. Reads one JSON object per line from
    stdin, dispatches by op, writes the response envelope to stdout.
    Exits when stdin closes or a shutdown op is processed."""
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
            _emit({"error": str(exc)})
            continue
        op = msg["op"]
        if op == "ready":
            _emit(_handle_ready())
        elif op == "synthesize":
            _emit(_handle_synthesize(msg))
        elif op == "shutdown":
            _emit(_handle_shutdown())
            return
        else:
            _emit({"error": f"unknown op: {op}"})


def main(argv: list[str]) -> int:
    if not _detect_adapter_mode(argv):
        p = _ap.ArgumentParser(prog="speech_tts_adapter")
        p.add_argument("--probe-artifact", action="store_true",
                       help="Print the artifact gate status and exit.")
        p.add_argument("--sha256-text", metavar="TEXT",
                       help="Print SHA-256 of the given text (no audio).")
        args = p.parse_args(argv)
        if args.probe_artifact:
            print(json.dumps(_artifact_gate(), indent=2))
            return 0
        if args.sha256_text:
            print(f"sha256={hashlib.sha256(args.sha256_text.encode()).hexdigest()}")
            return 0
        p.print_help()
        return 2
    _run_loop()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))