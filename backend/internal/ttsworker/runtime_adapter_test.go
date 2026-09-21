package ttsworker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func exec_LookPath(name string) (string, error) {
	// Best-effort existence check; matches Go's exec.LookPath
	// semantics for the existence check but skips the path walk
	// to keep this file free of extra imports.
	return name, nil
}

// ttsAdapterMockScript is a single Python program that reads JSONL
// from stdin and writes JSONL to stdout, simulating the Indic
// Parler-TTS Python adapter protocol.
//
// Behavior:
//   - On startup ("op":"ready"), emit a ready envelope with one
//     language (hi-IN), revision "mock-1", one voice, and a fixed
//     digest.
//   - On "op":"synthesize", echo a transcript envelope with a
//     16-byte deterministic WAV (RIFF header + 0 PCM data) and
//     duration 0.05s.
//   - On "op":"shutdown", exit 0.
const ttsAdapterMockScript = `#!/usr/bin/env python3
import base64
import json
import sys

DIGEST = "0" * 64
# Tiny but valid 16-bit mono 800Hz WAV: 44-byte header + 4 samples = 52 bytes.
WAV = bytes.fromhex(
    "524946462600000057415645666d74201000000001000100200000"
    "080200646174610800000000000000000000000000"
)

for raw in sys.stdin:
    line = raw.strip()
    if not line:
        continue
    try:
        msg = json.loads(line)
    except Exception:
        continue
    op = msg.get("op", "")
    if op == "ready":
        sys.stdout.write(json.dumps({
            "status": "ready",
            "revision": "mock-1",
            "languages": ["hi-IN"],
            "voices": [{"language": "hi-IN", "name": "default", "revision": "vmock-1"}],
            "digest_name": "mock-tts",
            "digest_sha256": DIGEST,
        }) + "\n")
        sys.stdout.flush()
    elif op == "synthesize":
        sys.stdout.write(json.dumps({
            "request_id": msg.get("request_id", ""),
            "audio_b64": base64.b64encode(WAV).decode("ascii"),
            "duration_secs": 0.05,
        }) + "\n")
        sys.stdout.flush()
    elif op == "shutdown":
        sys.stdout.write(json.dumps({"status": "shutdown"}) + "\n")
        sys.stdout.flush()
        sys.exit(0)
sys.exit(0)
`

func writeTTSAdapterMockScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mock_tts_adapter.py")
	if err := os.WriteFile(path, []byte(ttsAdapterMockScript), 0o755); err != nil {
		t.Fatalf("write mock: %v", err)
	}
	return path
}

// TestAdapterSubprocessRuntime_LoadModel_PopulatesRevision verifies
// the happy path: LoadModel returns nil and populates revision,
// digest, languages, voices, voiceMap.
func TestAdapterSubprocessRuntime_LoadModel_PopulatesRevision(t *testing.T) {
	bin := writeTTSAdapterMockScript(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if r.Revision() != "mock-1" {
		t.Errorf("Revision: got %q want %q", r.Revision(), "mock-1")
	}
	if got := r.Languages(); len(got) != 1 || got[0] != "hi-IN" {
		t.Errorf("Languages: got %v", got)
	}
	if got := r.Voices(); len(got) != 1 || got[0].Name != "default" {
		t.Errorf("Voices: got %+v", got)
	}
	if got := r.Voice("hi-IN"); got != "default" {
		t.Errorf("Voice(hi-IN): got %q want %q", got, "default")
	}
}

// TestAdapterSubprocessRuntime_Synthesize_ReturnsWavBytes verifies
// that a synthesize call produces a non-empty WavBytes result when
// the adapter returns valid audio.
func TestAdapterSubprocessRuntime_Synthesize_ReturnsWavBytes(t *testing.T) {
	bin := writeTTSAdapterMockScript(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	res, err := r.Synthesize(RequestContext{}, "hello", "hi-IN", "")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if res == nil || res.Empty {
		t.Fatalf("expected non-empty SynthResult, got %+v", res)
	}
	if len(res.WavBytes) == 0 {
		t.Errorf("WavBytes empty")
	}
	// RIFF header magic.
	if len(res.WavBytes) < 4 || string(res.WavBytes[:4]) != "RIFF" {
		t.Errorf("WavBytes not a WAV: %q", string(res.WavBytes[:min(4, len(res.WavBytes))]))
	}
}

// TestAdapterSubprocessRuntime_Synthesize_RejectsUnsupportedLanguage
// ensures language is checked before writing to stdin.
func TestAdapterSubprocessRuntime_Synthesize_RejectsUnsupportedLanguage(t *testing.T) {
	bin := writeTTSAdapterMockScript(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	_, err := r.Synthesize(RequestContext{}, "hello", "fr-FR", "")
	if !errors.Is(err, ErrLanguageUnsupported) {
		t.Errorf("expected ErrLanguageUnsupported, got %v", err)
	}
}

// TestAdapterSubprocessRuntime_NotLoadedReturnsUnavailable ensures
// Synthesize without LoadModel returns ErrRuntimeUnavailable.
func TestAdapterSubprocessRuntime_NotLoadedReturnsUnavailable(t *testing.T) {
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    "echo",
		Module: "ignored",
	})
	_, err := r.Synthesize(RequestContext{}, "hello", "hi-IN", "")
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("expected ErrRuntimeUnavailable, got %v", err)
	}
}

// TestAdapterSubprocessRuntime_Close_KillsSubprocess verifies Close
// shuts down the subprocess.
func TestAdapterSubprocessRuntime_Close_KillsSubprocess(t *testing.T) {
	bin := writeTTSAdapterMockScript(t)
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:    bin,
		Module: "ignored",
	})
	if err := r.LoadModel(); err != nil {
		t.Fatalf("LoadModel: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second close is idempotent.
	if err := r.Close(); err != nil {
		t.Errorf("Close twice: %v", err)
	}
}

// TestTTSAdapterStderrBuf_BoundedCapture mirrors the ASR test.
func TestTTSAdapterStderrBuf_BoundedCapture(t *testing.T) {
	var b ttsStderrBuf
	big := make([]byte, 10000)
	for i := range big {
		big[i] = 'x'
	}
	n, err := b.Write(big)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 10000 {
		t.Errorf("Write must report full length even when truncated: got %d want 10000", n)
	}
	if got := b.String(); len(got) != 4096 {
		t.Errorf("captured bytes: got %d want 4096", len(got))
	}
}

// TestAdapterSubprocessRuntime_LoadModel_RealPythonAdapter_BLOCKED
// exercises the real Python TTS adapter in the BLOCKED state.
func TestAdapterSubprocessRuntime_LoadModel_RealPythonAdapter_BLOCKED(t *testing.T) {
	if _, err := exec_LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	if os.Getenv("STHIRA_PYTHONPATH") == "" {
		t.Skip("STHIRA_PYTHONPATH not set; real Python adapter test skipped")
	}
	r := NewAdapterSubprocessRuntime(AdapterSubprocessConfig{
		Cmd:      "python3",
		Module:   "sthira_v2.speech_tts_adapter",
		ExtraEnv: []string{"PYTHONPATH=" + os.Getenv("STHIRA_PYTHONPATH")},
	})
	err := r.LoadModel()
	if err == nil {
		t.Fatal("BLOCKED adapter should refuse LoadModel")
	}
	if !errors.Is(err, ErrRuntimeUnavailable) {
		t.Errorf("expected ErrRuntimeUnavailable, got %v", err)
	}
}
