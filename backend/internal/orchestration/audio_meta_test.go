package orchestration_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

// TestStageTTS_PropagatesReturnedSettingsNotRequestSide:
// the orchestrator must surface the WORKER's declared settings
// (the actual rate of the bytes), not the rate it requested.
// Otherwise the client would see a 16 kHz label on native-rate
// audio and the cache key would mismatch on the next request.
func TestStageTTS_PropagatesReturnedSettingsNotRequestSide(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.ApprovedTemplateSHA[contracts.TemplateDigestKey("welcome", "en-IN")] = orchestrationtest.DigestString("Welcome.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Welcome.", SyntheticOnly: false,
	})
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		key := "welcome"
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion,
				RequestID:     req.RequestID,
				DataVersion:   req.ScopedContext.DataVersion,
				Status:        contracts.StatusOK,
				Intent:        orchestrationtest.IntentPtr(contracts.IntentListDestinations),
				Language:      req.Transcript.Language,
				Actions:       []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{"FAC-DEMO-1"}}},
				SpeechKey:     &key,
			},
			ModelRevision: "r0",
		}, nil
	})

	// The fake's audio is 16 kHz mono PCM16. We override Settings
	// to claim 22050 Hz on the response — this represents a real
	// worker that runs natively at 22050 Hz. The orchestrator must
	// surface 22050 in the PipelineAudio envelope (not 16000 from
	// the request side), and the bytes must validate at 22050.
	wav22050 := makeTestWAV(t, 22050, 1)
	tts.SetSynthesizeHook(func(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       base64.StdEncoding.EncodeToString(wav22050),
			ContentType:    "audio/wav",
			ChecksumSHA256: sha256Hex(wav22050),
			ModelRevision:  "r0",
			VoiceRevision:  "v0",
			Settings: contracts.TTSSynthesisSettings{
				SampleRate: 22050, BitDepth: 16, Channels: 1,
			},
		}, nil
	})

	req := transcriptPipelineRequest("JTEST", "en-IN", "show me options")
	req.Render.Kind = contracts.PipelineRenderTTS
	out, err := o.Process(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if out.Audio == nil {
		t.Fatalf("Audio envelope missing")
	}
	if got := out.Audio.Settings.SampleRate; got != 22050 {
		t.Errorf("PipelineAudio.Settings.SampleRate = %d, want 22050 (worker's truth, not 16000 from request)", got)
	}
}

// TestStageTTS_RejectsMismatchedSettings: when the worker's
// claimed Settings do not match the actual RIFF header, the
// orchestrator fails closed with an INTERNAL error. This is the
// "Never declare 16 kHz while delivering native-rate audio" guard.
func TestStageTTS_RejectsMismatchedSettings(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.ApprovedTemplateSHA[contracts.TemplateDigestKey("welcome", "en-IN")] = orchestrationtest.DigestString("Welcome.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Welcome.", SyntheticOnly: false,
	})
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(ctx context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		key := "welcome"
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion,
				RequestID:     req.RequestID,
				DataVersion:   req.ScopedContext.DataVersion,
				Status:        contracts.StatusOK,
				Intent:        orchestrationtest.IntentPtr(contracts.IntentListDestinations),
				Language:      req.Transcript.Language,
				Actions:       []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{"FAC-DEMO-1"}}},
				SpeechKey:     &key,
			},
			ModelRevision: "r0",
		}, nil
	})

	// Worker claims 22050 Hz in Settings but the bytes are 16 kHz.
	wav16000 := makeTestWAV(t, 16000, 1)
	tts.SetSynthesizeHook(func(ctx context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       base64.StdEncoding.EncodeToString(wav16000),
			ContentType:    "audio/wav",
			ChecksumSHA256: sha256Hex(wav16000),
			ModelRevision:  "r0",
			VoiceRevision:  "v0",
			Settings: contracts.TTSSynthesisSettings{
				SampleRate: 22050, BitDepth: 16, Channels: 1,
			},
		}, nil
	})

	req := transcriptPipelineRequest("JTEST", "en-IN", "show me options")
	req.Render.Kind = contracts.PipelineRenderTTS
	out, err := o.Process(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Process returned top-level error: %v", err)
	}
	if out.Audio != nil {
		t.Fatalf("mismatched Settings vs RIFF must suppress audio; got audio=%+v", out.Audio)
	}
	if len(out.Stages) == 0 || !strings.Contains(out.Stages[0].Reason, "RIFF header") {
		t.Fatalf("Stages must include a RIFF-header-mismatch failure, got %+v", out.Stages)
	}
}

// orchestration package helpers below — used by the orchestration_test
// tests above. Kept in this file so the orchestration_test package can
// drive audio metadata behavior end-to-end.

func makeTestWAV(t *testing.T, rate int, seconds float64) []byte {
	t.Helper()
	samples := int(float64(rate) * seconds)
	dataLen := samples * 2
	total := 36 + dataLen
	b := make([]byte, 44+dataLen)
	copy(b[0:4], "RIFF")
	b[4], b[5], b[6], b[7] = byte(total), byte(total>>8), byte(total>>16), byte(total>>24)
	copy(b[8:12], "WAVE")
	copy(b[12:16], "fmt ")
	b[16], b[17], b[18], b[19] = 16, 0, 0, 0
	b[20], b[21] = 1, 0
	b[22], b[23] = 1, 0
	b[24], b[25], b[26], b[27] = byte(rate), byte(rate>>8), byte(rate>>16), byte(rate>>24)
	b[28], b[29], b[30], b[31] = byte(rate*2), byte((rate*2)>>8), byte((rate*2)>>16), byte((rate*2)>>24)
	b[32], b[33] = 2, 0
	b[34], b[35] = 16, 0
	copy(b[36:40], "data")
	b[40], b[41], b[42], b[43] = byte(dataLen), byte(dataLen>>8), byte(dataLen>>16), byte(dataLen>>24)
	return b
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	const hexchars = "0123456789abcdef"
	out := make([]byte, len(sum)*2)
	for i, c := range sum {
		out[2*i] = hexchars[c>>4]
		out[2*i+1] = hexchars[c&0x0F]
	}
	return string(out)
}

// _silence pinpoints an unused-import for go vet.
var _ = orchestration.DefaultLimits
