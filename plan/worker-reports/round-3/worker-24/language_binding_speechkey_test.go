package orchestration_test

import (
	"context"
	"encoding/base64"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

func TestLanguageBinding_WorkerMisattestedSpeechKeyRejected(t *testing.T) {
	r := newLangRig(t)
	k := destKey
	r.propose(langHI, &k, contracts.IntentListDestinations, choices())
	wav16k := makeTestWAV(t, 16000, 0.1)
	r.tts.SetSynthesizeHook(func(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      "wrong_speech_key", // attestation mismatch: wrong key, right language
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       base64.StdEncoding.EncodeToString(wav16k),
			ContentType:    "audio/wav",
			ChecksumSHA256: sha256Hex(wav16k),
			ModelRevision:  "fake",
			VoiceRevision:  "fake",
			Settings:       contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1},
		}, nil
	})
	out := r.run(t, langHI)
	if out.Audio != nil {
		t.Fatalf("audio must be nil when worker misattests speech_key: got audio=%+v", out.Audio)
	}
	if !hasStage(out, orchestration.StageTTS) {
		t.Fatalf("expected tts stage failure, got stages=%v", out.Stages)
	}
}
