#!/usr/bin/env python3
"""
TTS timeout behavior probe.
Tests that the TTS worker correctly handles:
1. Per-call timeout (15s) on slow subprocess responses
2. Cancellation cleanly deregisters pending requests
3. Subsequent calls recover correctly after timeout/cancel

The ttsworker/ subdir has its own go.mod (go 1.23) so tests must
run from inside that directory.
"""
import subprocess
import sys
import os

TTSWORKER_DIR = "/Users/apple/Documents/Projects/MonitoringZ/backend/internal/ttsworker"

def run_test(binary, pattern):
    result = subprocess.run(
        ["go", "test", "-v", "-run", pattern, "./"],
        cwd=TTSWORKER_DIR,
        capture_output=True,
        text=True,
        timeout=60,
    )
    print(result.stdout)
    if result.stderr:
        print(result.stderr, file=sys.stderr)
    if result.returncode != 0:
        print(f"FAIL: {pattern} (exit {result.returncode})")
        return False
    print(f"PASS: {pattern}")
    return True

def main():
    tests = [
        ("cancel-then-retry", "TestB3_TTS_CancelThenRetry"),
        ("write deadline + bounded close", "TestTTSIPC_WriteDeadlineAndBoundedClose"),
        ("malformed resets demux", "TestTTSIPC_MalformedResets"),
        ("subprocess exit releases pending", "TestB3_TTS_SubprocessExitReleasesPending"),
    ]
    all_passed = True
    for name, test in tests:
        if not run_test("go", test):
            all_passed = False
    if not all_passed:
        sys.exit(1)
    print("\nAll TTS timeout/procedure tests passed.")
    sys.exit(0)

if __name__ == "__main__":
    main()
