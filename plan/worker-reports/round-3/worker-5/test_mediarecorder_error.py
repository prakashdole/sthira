"""
Regression: MediaRecorder.onerror is handled gracefully.

Artifact: plan/worker-reports/round-3/worker-5/test_mediarecorder_error.py
NOT_RUN — deferred by user. Requires Playwright and a running dev server.

Setup:
    pip install playwright
    playwright install chromium
    cd frontend/v2 && npm run dev

Run:
    python tests/test_mediarecorder_error.py

What it tests:
    1. App loads and voice console opens.
    2. Fake MediaRecorder that fires an error event is injected before getUserMedia.
    3. User clicks the mic/start-recording button.
    4. After the fake error fires, the app should call cancelRecording()
       and show voiceFeedbackKey='micStopped' feedback (visible in the DOM).
    5. The network is inspected: NO audio POST should be made to /api/v3/voice/process
       because shouldDropRecordedAudio(recordingCancelled=true) should drop it in onstop.
    6. Recovery: after the error is handled, the user can still interact with the UI.

Expected failure BEFORE the patch:
    - voiceFeedbackKey stays 'recording' (never updated) because onerror is not set.
    - No error feedback shown in the UI.
    - The timeout still fires and may attempt to send an empty/silent audio blob.

Expected pass AFTER the patch:
    - onerror fires, calls cancelRecording(), sets voiceFeedbackKey='micStopped'.
    - UI updates to show the mic-stopped feedback.
    - No audio sent to backend.
"""

import asyncio
import re
from playwright.sync_api import sync_playwright, Page, expect


APP_URL = "http://localhost:5173"
TIMEOUT_MS = 10_000


def run() -> None:
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context()
        page = context.new_page()

        # Capture network requests to verify no audio is sent
        audio_requests: list = []

        def on_request(request):
            if "/api/v3/voice/process" in request.url:
                audio_requests.append(request)

        page.on("request", on_request)

        # 1. Open app and wait for it to be ready
        page.goto(APP_URL, wait_until="networkidle", timeout=TIMEOUT_MS)

        # 2. Inject fake MediaRecorder that fires an error immediately on start()
        # The fake:
        #   - supports the same MIME types as the app checks
        #   - fires onerror 100ms after start() is called
        #   - does NOT fire onstop (the error path — onstop would fire after stop())
        fake_recorder_code = """
        (function() {
            const OriginalMediaRecorder = window.MediaRecorder;
            let _onerror = null;
            let _onstop = null;
            let _ondataavailable = null;
            let _state = 'inactive';

            function FakeMediaRecorder(stream, options) {
                this.stream = stream;
                this.mimeType = options.mimeType || 'audio/webm';
                this.state = 'inactive';
                this.onerror = null;
                this.onstop = null;
                this.ondataavailable = null;
            }
            FakeMediaRecorder.isTypeSupported = function(mime) {
                return mime === 'audio/webm;codecs=opus' || mime === 'audio/webm';
            };
            FakeMediaRecorder.prototype.start = function() {
                const self = this;
                this.state = 'recording';
                // Fire error after a short delay — simulates hardware/encoding failure
                setTimeout(function() {
                    self.state = 'inactive';
                    if (self.onerror) {
                        self.onerror(new MediaRecorderErrorEvent('error', {
                            error: { name: 'UnknownError', message: 'Simulated recorder failure' }
                        }));
                    }
                }, 100);
            };
            FakeMediaRecorder.prototype.stop = function() {
                this.state = 'inactive';
                if (this.onstop) this.onstop();
            };
            FakeMediaRecorder.prototype.requestData = function() {};

            window.MediaRecorder = FakeMediaRecorder;
            window._fakeRecorderReady = true;
        })();
        """
        page.evaluate(fake_recorder_code)

        # Verify injection worked
        assert page.evaluate("window._fakeRecorderReady") is True, "Fake MediaRecorder not injected"

        # 3. Open the voice console
        page.click('[data-action="voice-open"]', timeout=TIMEOUT_MS)
        page.wait_for_selector('.voice-console', state="attached", timeout=TIMEOUT_MS)

        # 4. Click the listen/record button to trigger getUserMedia + MediaRecorder
        page.click('[data-action="voice-listen"]', timeout=TIMEOUT_MS)

        # 5. Wait for the error feedback to appear in the DOM
        #    The voiceFeedbackKey should change from 'recording' to 'micStopped'
        #    This renders as part of the voice console — check for the micStopped state
        try:
            page.wait_for_timeout(500)  # give time for the error to propagate
            # The voice console should still be open and responsive
            console_visible = page.is_visible(".voice-console")
            assert console_visible, "Voice console should remain visible after error"
        except Exception as exc:
            raise AssertionError(
                f"UI became unresponsive or console closed after MediaRecorder error. "
                f"onerror handler may not be wired. Audio requests made: {audio_requests}"
            ) from exc

        # 6. Verify the error feedback key is reflected (micStopped, not 'recording')
        #    The UI updates voiceFeedbackKey via render() which is called in onerror.
        #    We check that the listening indicator is no longer shown.
        is_still_listening = page.is_visible(".voice-stage.is-listening")
        assert not is_still_listening, (
            "Voice stage still shows 'is-listening' class after error. "
            "This means voiceFeedbackKey was not updated to 'micStopped'. "
            "Expected: .is-listening removed after cancelRecording()."
        )

        # 7. Verify no audio POST was made to the voice process endpoint
        #    (The onstop should have been called by cancelRecording() and shouldDropRecordedAudio
        #     should have returned true because recordingCancelled was set)
        audio_post_count = len(audio_requests)
        assert audio_post_count == 0, (
            f"Expected ZERO audio POSTs to /api/v3/voice/process but got {audio_post_count}. "
            f"The partial/cancelled audio should have been dropped by shouldDropRecordedAudio "
            f"because cancelRecording sets recordingCancelled=true before calling stop()."
        )

        # 8. Verify the UI is still functional after the error (recovery)
        close_button = page.query_selector('[data-action="voice-close"]')
        assert close_button is not None, "Voice close button should still be in DOM"
        close_button.click()
        page.wait_for_timeout(200)
        console_hidden = not page.is_visible(".voice-console")
        assert console_hidden, "Voice console should close after clicking X"

        browser.close()
        print("PASS: MediaRecorder error handled correctly — no spurious audio sent, UI updated")


if __name__ == "__main__":
    run()
