"""JSONL adapter for IndicConformer ASR.

This module implements the line-delimited JSON protocol expected by
backend/internal/asrworker/runtime_adapter.go. It is invoked as:

    python3 -m sthira_v2.speech_asr_adapter --adapter-mode

The adapter reads JSONL requests from stdin and emits JSONL responses
on stdout. Each request has an "op" field:

    {"op":"ready"}          →  ready envelope with revision/languages/digest
    {"op":"transcribe",     →  {"request_id":..., "text":..., "confidence":...}
     "request_id":..., ...}
    {"op":"shutdown"}       →  graceful exit

Honesty gate (BLOCKED_EXTERNAL semantics):

Real inference requires the IndicConformer-600M-Multi model weights,
which are not present in this revision. Until authorized artifacts
land (per plan/open-decisions.md O03), the adapter refuses inference
on every "transcribe" request and reports ready=false equivalent:

    {"status":"blocked","reason":"artifact gate not ready"}

The orchestrator surfaces this as ErrRuntimeUnavailable. No canned
transcript is ever emitted.

The protocol layer, the JSON envelope, the request-ID correlation
and the bounded-error handling are fully implemented here so the Go
side can be tested against a real subprocess. The "ready" envelope
returned when artifacts are unavailable advertises an empty language
list and an empty revision so the Go runtime stays in the not-loaded
state by design.

When artifacts become available (post O03 resolution), the
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


INDIC_CONFORMER_MODEL = "ai4bharat/indic-conformer-600m-multilingual"
TARGET_SAMPLE_RATE = 16000
PROTOCOL_VERSION = "asr-adapter-1"


def _detect_adapter_mode(argv: list[str]) -> bool:
    return "--adapter-mode" in argv


def _artifact_gate() -> dict[str, Any]:
    """Honest artifact gate. Returns BLOCKED until weights are installed.

    The single seam for wiring real model loading is here: replace
    the body with the loader that materializes an AI4Bharat
    IndicConformer-600M-Multi model from the pinned local artifact
    directory. Until then, this is the truthful "blocked real
    inference" state the Stage 6 review demanded.
    """
    return {
        "model_id": INDIC_CONFORMER_MODEL,
        "revision": "",
        "languages": [],
        "digest_name": "",
        "digest_sha256": "",
        "state": "BLOCKED",
        "reason": "real inference adapter: artifact gate not ready (O03)",
    }


def _emit(obj: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps(obj, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def _handle_ready() -> dict[str, Any]:
    gate = _artifact_gate()
    # When no artifact, advertise the BLOCKED state directly. The Go
    # runtime's LoadModel populates revision/languages only from a
    # {"status":"ready",...} envelope; BLOCKED is a different signal
    # that keeps the worker in the not-loaded state by design.
    return {
        "status": "blocked",
        "model_id": gate["model_id"],
        "protocol": PROTOCOL_VERSION,
        "reason": gate["reason"],
    }


def _handle_transcribe(msg: dict[str, Any]) -> dict[str, Any]:
    rid = msg.get("request_id", "")
    # We never produce a transcript when the artifact gate is
    # blocked. Returning an error envelope preserves the IPC and
    # surfaces BLOCKED to the Go runtime as ErrRuntimeUnavailable.
    return {
        "request_id": rid,
        "error": "artifact gate blocked: real inference not wired (O03)",
    }


def _handle_shutdown() -> dict[str, Any]:
    return {"status": "shutdown"}


def _decode_samples(samples_b64: str) -> bytes:
    """Decode base64-encoded little-endian float32 samples."""
    try:
        return base64.b64decode(samples_b64.encode("ascii"))
    except Exception as exc:
        raise ValueError(f"samples_b64 decode: {exc}") from exc


def _verify_envelope_shape(msg: dict[str, Any]) -> None:
    """Structural check on inbound request envelopes. The Go side
    already enforces this, but a defensive check here keeps the
    adapter honest if it is ever driven directly."""
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
            _emit({"error": str(exc), "request_id": msg.get("request_id", "")})
            continue
        op = msg["op"]
        if op == "ready":
            _emit(_handle_ready())
        elif op == "transcribe":
            _emit(_handle_transcribe(msg))
        elif op == "shutdown":
            _emit(_handle_shutdown())
            return
        else:
            _emit({"error": f"unknown op: {op}"})


def main(argv: list[str]) -> int:
    if not _detect_adapter_mode(argv):
        # When invoked without --adapter-mode, behave as a module CLI
        # for ad-hoc testing.
        p = _ap.ArgumentParser(prog="speech_asr_adapter")
        p.add_argument("--probe-artifact", action="store_true",
                       help="Print the artifact gate status and exit.")
        p.add_argument("--decode-samples", metavar="B64",
                       help="Decode a base64 float32 LE buffer and print the SHA-256.")
        args = p.parse_args(argv)
        if args.probe_artifact:
            print(json.dumps(_artifact_gate(), indent=2))
            return 0
        if args.decode_samples:
            raw = _decode_samples(args.decode_samples)
            print(f"sha256={hashlib.sha256(raw).hexdigest()} bytes={len(raw)}")
            return 0
        p.print_help()
        return 2
    _run_loop()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))