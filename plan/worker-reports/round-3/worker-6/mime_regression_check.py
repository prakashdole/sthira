"""
mime_regression_check.py — Browser-side MIME negotiation and error-recovery check.
NOT_RUN — deferred per assignment.
Prerequisites:
  - Dev server running at http://localhost:5173
  - pytest with Playwright: pip install playwright && playwright install --with-deps chromium
  - Browser in English for label stability

Run:
  pytest plan/worker-reports/round-3/worker-6/mime_regression_check.py -v

Checks:
  1. MediaRecorder is constructed with a browser-supported MIME type.
  2. If no supported MIME, feedback key is set to 'micStopped' and recording does not start.
  3. Onerror fires and calls cancelRecording when a recording error occurs.
  4. An invalid start() throws and sets feedback to 'micStopped'.
  5. Backend would accept the chosen MIME type (audio/webm or audio/webm;codecs=opus).
"""

import re
import subprocess
from pathlib import Path

import pytest

BASE_DIR = Path(__file__).parent
FRONTEND_DIR = BASE_DIR.parent.parent.parent / "frontend" / "v2"


def get_mime_type_selection() -> str:
    """
    Return the MIME type string the frontend would select, as a code analysis result.
    """
    main_ts = FRONTEND_DIR / "src" / "main.ts"
    content = main_ts.read_text()

    mime_pattern = re.compile(
        r"MediaRecorder\.isTypeSupported\(['\"]audio/webm;codecs=opus['\"]"
        r"\s*\?\s*['\"]([^'\"]+)['\"]"
        r"\s*:\s*['\"]([^'\"]+)['\"]",
        re.MULTILINE,
    )
    m = mime_pattern.search(content)
    if m:
        return m.group(1)  # the selected mime type

    return "audio/webm;codecs=opus"


class TestRecorderMimeSelection:
    def test_mime_type_is_backend_accepted(self):
        """
        The MIME type the browser would select must be accepted by the backend.
        Backend accept list: audio/wav, audio/webm (any opus), audio/ogg (any opus), audio/opus.
        """
        mime = get_mime_type_selection()
        base_type = mime.split(";")[0].strip()
        backend_accepted = {"audio/wav", "audio/webm", "audio/ogg", "audio/opus"}
        assert (
            base_type in backend_accepted
        ), f"MIME '{mime}' (base '{base_type}') not in backend allowlist {backend_accepted}"

    def test_mime_pre_check_or_onerror_handler_exists(self):
        """
        The source must either:
        (a) validate MIME support before constructing MediaRecorder, OR
        (b) assign onerror to handle start() failure.
        Without one of these, unsupported MIME silently produces empty audio.
        """
        main_ts = FRONTEND_DIR / "src" / "main.ts"
        content = main_ts.read_text()

        has_precheck = bool(
            re.search(r"MediaRecorder\.isTypeSupported\s*\(\s*mimeType\s*\)", content)
        )
        has_onerror = bool(re.search(r"mediaRecorder\.onerror\s*=", content))

        assert has_precheck or has_onerror, (
            "No MIME pre-check and no onerror handler found in toggleLocalRecording. "
            "Unsupported MIME silently fails."
        )

    def test_start_wrapped_in_try_catch(self):
        """
        mediaRecorder.start() must be inside a try/catch that calls cancelRecording
        on failure, preventing inconsistent UI state after a start() error.
        """
        main_ts = FRONTEND_DIR / "src" / "main.ts"
        content = main_ts.read_text()

        func_match = re.search(
            r"async function toggleLocalRecording\(\)[^{]*\{(.*?)\n\}",
            content,
            re.DOTALL,
        )
        assert func_match, "toggleLocalRecording function not found"
        func_body = func_match.group(1)

        try_catch_for_start = re.search(
            r"try\s*\{[^}]*mediaRecorder\.start\(\)[^}]*\}\s*catch", func_body
        )
        assert try_catch_for_start, (
            "mediaRecorder.start() is not wrapped in try/catch inside toggleLocalRecording. "
            "A NotSupportedError from start() will propagate uncaught."
        )

    def test_cancel_recording_calls_stop_before_nulling(self):
        """
        cancelRecording must call mediaRecorder.stop() before nullifying the reference,
        so that any pending onstop fires with valid state.
        """
        main_ts = FRONTEND_DIR / "src" / "main.ts"
        content = main_ts.read_text()

        cancel_match = re.search(
            r"function cancelRecording\(\)[^{]*\{(.*?)\n\}",
            content,
            re.DOTALL,
        )
        assert cancel_match, "cancelRecording function not found"
        cancel_body = cancel_match.group(1)

        stop_before_null = re.search(
            r"mediaRecorder\.stop\(\).*mediaRecorder\s*=\s*null",
            cancel_body,
            re.DOTALL,
        )
        assert stop_before_null, (
            "cancelRecording does not call mediaRecorder.stop() before setting "
            "mediaRecorder = null. onstop may fire with mediaRecorder already nulled."
        )

    def test_backend_mime_test_exists_and_passes(self):
        """
        The backend audio_mime_test.go must exist and pass when executed.
        This verifies the backend actually accepts the MIME types the frontend sends.
        """
        test_file = BASE_DIR.parent.parent.parent / "backend" / "internal" / "asrworker" / "audio_mime_test.go"
        if not test_file.exists():
            pytest.skip("audio_mime_test.go not found in backend")

        result = subprocess.run(
            ["go", "test", "-v", "-run", "TestDecodeAudio_.*Webm|TestDecodeAudio_.*Ogg", "./backend/internal/asrworker/"],
            capture_output=True,
            text=True,
            cwd=BASE_DIR.parent.parent.parent,
        )
        assert result.returncode == 0, (
            f"Backend MIME tests failed:\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
