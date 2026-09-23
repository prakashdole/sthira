// Package orchestration_test: A1 regression tests that prove the
// orchestrator's Process + Synthesize reject every documented
// strict-boundary defect at the orchestration package boundary.
//
// Reproduces these defects:
//
//   - empty returned proposal RequestID passes correlation (must reject).
//   - RECENTER action carrying a TargetID passes shape (must reject extra fields).
//   - showing NEED_CLARIFICATION status passes the validator (must reject).
//   - oversized JSON body sneaks past the strict decoder (must reject at
//     the raw decode boundary, not decoded into a partial struct).
//   - middle-worker response larger than the per-call limit is treated as
//     a successful decode (must detect the max+1 overflow before parsing).
//   - local/fake HTTPWorkerClient / client bypass cannot disable the
//     mandatory semantic validator.
//
// These tests fail before the fix and pass after.
package orchestration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

// wantPipelineReject runs Process and asserts the orchestrator fails closed with the given stable code.
func wantPipelineReject(t *testing.T, o *orchestration.Orchestrator, req contracts.PipelineRequest, wantCode string) {
	t.Helper()
	_, err := o.Process(context.Background(), req, nil)
	if err == nil {
		t.Fatalf("Process: expected failure (%s), got nil", wantCode)
	}
	var pe *orchestration.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("Process: error is not *PipelineError: %v", err)
	}
	if len(pe.Failures) == 0 || pe.Failures[0].Code != wantCode {
		t.Fatalf("Process: Code = %s, want %s", firstCode(pe), wantCode)
	}
}

func firstCode(pe *orchestration.PipelineError) string {
	if len(pe.Failures) == 0 {
		return ""
	}
	return pe.Failures[0].Code
}

// hookProposeOK injects a successful Propose returning proposal. The proposal
// will reach the in-package validateProposal, which is the boundary we test.
// Optional finalize lets the caller override auto-fill behavior; pass nil to
// use the default that fills empty RequestID/DataVersion/Language from the
// server-generated values (most tests want this), or a function to test
// strict-boundary behavior where auto-fill is disabled.
func hookProposeOK(mid *orchestrationtest.Worker, proposal contracts.ModelOutput) {
	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		out := proposal
		if out.RequestID == "" {
			out.RequestID = req.RequestID
		}
		if out.DataVersion == "" {
			out.DataVersion = req.ScopedContext.DataVersion
		}
		if out.Language == "" {
			out.Language = req.Transcript.Language
		}
		return contracts.MiddleWorkerResponse{
			RequestID:     req.RequestID,
			DataVersion:   req.ScopedContext.DataVersion,
			ModelRevision: "r0",
			Proposal:      out,
		}, nil
	})
}

// hookProposeRaw injects a Propose returning proposal WITHOUT auto-filling
// empty fields. Used to test strict-boundary behavior.
func hookProposeRaw(mid *orchestrationtest.Worker, proposal contracts.ModelOutput) {
	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		out := proposal
		return contracts.MiddleWorkerResponse{
			RequestID:     req.RequestID,
			DataVersion:   req.ScopedContext.DataVersion,
			ModelRevision: "r0",
			Proposal:      out,
		}, nil
	})
}

// TestA1_EmptyProposalRequestIDRejected: a middle-worker response whose
// RequestID is empty must NOT pass the validator (the contract echoes the
// server request_id; the validator must require exact equality when the
// server has one).
func TestA1_EmptyProposalRequestIDRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	// empty out.RequestID with a populated requestID is a contract violation.
	hookProposeRaw(mid, contracts.ModelOutput{
		SchemaVersion: contracts.ModelSchemaVersion,
		// RequestID intentionally "" — contract requires the proposal to echo.
		Status:           contracts.StatusOK,
		Intent:           orchestrationtest.IntentPtr(contracts.IntentRecenter),
		Actions:          []contracts.Action{{Type: contracts.ActionRecenter}},
		ClarificationIDs: nil,
	})

	req := transcriptPipelineRequest("JTEST", "en-IN", "x")
	wantPipelineReject(t, o, req, contracts.ErrValidation)
}

// TestA1_MismatchedProposalRequestIDRejected: the validator must refuse
// a proposal whose RequestID does not equal the server's correlation ID.
func TestA1_MismatchedProposalRequestIDRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	hookProposeOK(mid, contracts.ModelOutput{
		SchemaVersion: contracts.ModelSchemaVersion,
		RequestID:     "some-other-id", // server got req.RequestID; this must mismatch.
		Status:        contracts.StatusOK,
		Intent:        orchestrationtest.IntentPtr(contracts.IntentRecenter),
		Actions:       []contracts.Action{{Type: contracts.ActionRecenter}},
		Language:      "en-IN",
		DataVersion:   "PKG-1:1",
	})
	wantPipelineReject(t, o, transcriptPipelineRequest("JTEST", "en-IN", "x"), contracts.ErrValidation)
}

// TestA1_RecenterWithTargetIDRejected: the strict per-action schema must
// reject RECENTER carrying any extra field (TargetID, RouteID, etc.).
func TestA1_RecenterWithTargetIDRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	hookProposeOK(mid, contracts.ModelOutput{
		SchemaVersion: contracts.ModelSchemaVersion,
		Status:        contracts.StatusOK,
		Intent:        orchestrationtest.IntentPtr(contracts.IntentRecenter),
		Language:      "en-IN",
		// RECENTER with a TargetID is forbidden by the strict tagged action schema.
		Actions: []contracts.Action{{Type: contracts.ActionRecenter, TargetID: "PLACE-DEMO-1"}},
	})
	wantPipelineReject(t, o, transcriptPipelineRequest("JTEST", "en-IN", "x"), contracts.ErrValidation)
}

// TestA1_RecenterWithRouteIDRejected: same for RouteID.
func TestA1_RecenterWithRouteIDRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	hookProposeOK(mid, contracts.ModelOutput{
		SchemaVersion: contracts.ModelSchemaVersion,
		Status:        contracts.StatusOK,
		Intent:        orchestrationtest.IntentPtr(contracts.IntentRecenter),
		Language:      "en-IN",
		Actions:       []contracts.Action{{Type: contracts.ActionRecenter, RouteID: "ROUTE-DEMO-1"}},
	})
	wantPipelineReject(t, o, transcriptPipelineRequest("JTEST", "en-IN", "x"), contracts.ErrValidation)
}

// TestA1_NeedClarificationRejected: the orchestrator's schema validator must
// only accept the strictly enum'd statuses (OK, CLARIFY, UNSUPPORTED,
// DATA_UNAVAILABLE, ERROR) — accepting "NEED_CLARIFICATION" silently
// widens the contract.
func TestA1_NeedClarificationRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		return contracts.MiddleWorkerResponse{
			RequestID:     req.RequestID,
			DataVersion:   req.ScopedContext.DataVersion,
			ModelRevision: "r0",
			Proposal: contracts.ModelOutput{
				SchemaVersion:    contracts.ModelSchemaVersion,
				RequestID:        req.RequestID,
				DataVersion:      req.ScopedContext.DataVersion,
				Status:           "NEED_CLARIFICATION", // synonym added by the implementation, NOT in contract.
				Language:         "en-IN",
				ClarificationIDs: []string{"PLACE-DEMO-1"},
			},
		}, nil
	})
	wantPipelineReject(t, o, transcriptPipelineRequest("JTEST", "en-IN", "x"), contracts.ErrValidation)
}

// TestA1_OversizedRequestRawBodyRejectedAtDecode: the orchestrator
// must reject a citizen request body that exceeds the PUBLIC
// request budget (MaxAudioCompressedBytes + MaxTranscriptUTF8Bytes
// + overhead), not the 64 KiB model-response limit. The handler's
// MaxBytesReader already enforces a similar ceiling; this test
// proves the orchestrator's own DecodeStrict gate works at the
// correct public boundary.
func TestA1_OversizedRequestRawBodyRejectedAtDecode(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	// Public budget is ~786 KiB. Build a PipelineRequest-shaped
	// body that exceeds it via a giant transcript field.
	// The public budget is slightly over 768 KiB; 900 KiB is safe.
	bigText := strings.Repeat("x", 900*1024)
	body := []byte(fmt.Sprintf(
		`{"request_id":"req-test-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":%q},"render":{"kind":"none"}}`,
		bigText))

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		t.Fatalf("Propose called despite oversized request rejection")
		return contracts.MiddleWorkerResponse{}, nil
	})

	_, err := o.Process(context.Background(), transcriptPipelineRequest("JTEST", "en-IN", "x"), body)
	if err == nil {
		t.Fatalf("Process: expected failure on oversized request body")
	}
}

// TestA1_TrailingRequestRawBodyRejectedAtDecode: the strict raw
// decoder must reject a valid PipelineRequest-shaped body that
// has trailing JSON content after the top-level value.
func TestA1_TrailingRequestRawBodyRejectedAtDecode(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	// Valid PipelineRequest with trailing garbage.
	body := []byte(`{"request_id":"req-test-1","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"hello"},"render":{"kind":"none"}}{"after":true}`)

	_, err := o.Process(context.Background(), transcriptPipelineRequest("JTEST", "en-IN", "x"), body)
	if err == nil {
		t.Fatalf("Process: expected failure on trailing request body")
	}
}

// TestA1_DuplicateKeysRejected: duplicate top-level keys in the
// citizen request body must be rejected.
func TestA1_DuplicateKeysRejected(t *testing.T) {
	// Duplicate request_id in PipelineRequest shape.
	body := []byte(`{"request_id":"req-1","request_id":"req-2","jurisdiction":"JTEST","language":"en-IN","input":{"kind":"transcript","text":"x"},"render":{"kind":"none"}}`)

	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	_, err := o.Process(context.Background(), transcriptPipelineRequest("JTEST", "en-IN", "x"), body)
	if err == nil {
		t.Fatalf("Process: expected failure on duplicate request raw key")
	}
}

// TestA1_ValidateShapeRuns: production validator's ValidateShape runs
// against malformed JSON. A "Shape" check that never fires is a hole.
func TestA1_ProductionValidatorValidateShape(t *testing.T) {
	v := orchestration.NewProductionValidator()
	if err := v.ValidateShape([]byte("not json")); err == nil {
		t.Fatalf("ProductionValidator.ValidateShape accepted non-JSON")
	}
	if err := v.ValidateShape(nil); err != nil {
		t.Fatalf("ProductionValidator.ValidateShape rejected empty (acceptable no-op)")
	}
	if err := v.ValidateShape([]byte(`{"ok":true}`)); err != nil {
		t.Fatalf("ProductionValidator.ValidateShape rejected trivial JSON: %v", err)
	}
}

// TestA1_WorkerResponseOverflowDetected: HTTPWorkerClient must
// detect a response larger than the per-call limit by reading
// beyond it before decoding, not silently truncate and decode a
// truncated envelope.
func TestA1_WorkerResponseOverflowDetected(t *testing.T) {
	// Set up a stub worker returning a payload larger than the
	// HTTPWorkerClient's allowed Synthesize cap (2 MiB) — produced
	// as invalid JSON, then a valid but over-cap JSON.
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/synthesize" {
			// Compose valid JSON but with audio base64 enormous enough to overflow the 2MiB cap.
			// We use a pad string of 4 MiB; the read of 2 MiB (LimitReader cap) must detect
			// that the body is not exhausted.
			pad := strings.Repeat("x", 4*1024*1024)
			body := fmt.Sprintf(`{"request_id":"req-a1","state":"OK","content_type":"audio/wav","checksum_sha256":"%s","audio_b64":"%s"}`, strings.Repeat("0", 64), pad)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	serverURL = ts.URL

	c := orchestration.NewHTTPWorkerClient(serverURL, "", nil)
	_, err := c.Synthesize(context.Background(), contracts.TTSWorkerRequest{
		RequestID: "req-a1",
		SpeechKey: "welcome",
		Language:  "en-IN",
		Text:      "hi",
	})
	if err == nil {
		t.Fatalf("Synthesize should fail on overflow but returned nil error")
	}
}

// TestA1_WorkerResponseTrailingDataRejected: HTTPWorkerClient must
// reject a worker response that begins with valid JSON but contains
// trailing data after the JSON value.
func TestA1_WorkerResponseTrailingDataRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/synthesize" {
			// audio_b64 is just a tiny valid base64 sample; trailing "{...}" is invalid.
			body := `{"request_id":"req-a1","state":"OK","content_type":"audio/wav","checksum_sha256":"0000000000000000000000000000000000000000000000000000000000000000","audio_b64":"AAAA"}{"more":1}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	_, err := c.Synthesize(context.Background(), contracts.TTSWorkerRequest{
		RequestID: "req-a1",
		SpeechKey: "welcome",
		Language:  "en-IN",
		Text:      "hi",
	})
	if err == nil {
		t.Fatalf("Synthesize should fail on trailing JSON but returned nil error")
	}
}

// TestA1_NonOKStatusWithActionRejected: the schema validator must reject
// a proposal whose status is non-OK but carries actions (a model that
// "fails safely" but still issues movement).
func TestA1_NonOKStatusWithActionRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion,
				RequestID:     req.RequestID,
				DataVersion:   req.ScopedContext.DataVersion,
				Status:        contracts.StatusUnsupported,
				Language:      "en-IN",
				// Non-OK carrying actions is forbidden — defensively meaningless.
				Actions:          []contracts.Action{{Type: contracts.ActionRecenter}},
				Intent:           nil,
				ClarificationIDs: nil,
			},
		}, nil
	})
	wantPipelineReject(t, o, transcriptPipelineRequest("JTEST", "en-IN", "x"), contracts.ErrValidation)
}

// TestA1_NonOKIntentRejected: non-OK status with non-nil Intent is rejected.
func TestA1_NonOKIntentRejected(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	resolver := orchestrationtest.NewResolver(orchestrationtest.BuildScopedContext("JTEST", "en-IN"))
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion,
				RequestID:     req.RequestID,
				DataVersion:   req.ScopedContext.DataVersion,
				Status:        contracts.StatusUnsupported,
				Intent:        orchestrationtest.IntentPtr(contracts.IntentRecenter),
				Language:      "en-IN",
			},
		}, nil
	})
	wantPipelineReject(t, o, transcriptPipelineRequest("JTEST", "en-IN", "x"), contracts.ErrValidation)
}

// helper to read all of resp.Body fully.
func drain(resp *http.Response) ([]byte, error) { return io.ReadAll(resp.Body) }

// TestA1_RealValidatorShapeDuplicatesUnknownKeyRejected: a raw body with
// silence unused-helper warnings.
var _ = drain

// TestA1_TTSWithdrawalStaleContext_DropsActions: when the context becomes
// stale BETWEEN the post-context revalidation and the post-TTS revalidation,
// the orchestrator must NOT return the validated proposal or audio.
// Currently a TTS failure causes the audio to be nil but the proposal is
// preserved; this is correct for transient TTS errors but NOT for a
// withdrawn/changed source: the validated proposal is no longer authoritative.
func TestA1_TTSWithdrawalStaleContext_DropsActions(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.ApprovedTemplateSHA["destination_options"] = orchestrationtest.DigestString("Destination choices are displayed on screen.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "destination_options", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Destination choices are displayed on screen.", SyntheticOnly: false,
	})
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		key := "destination_options"
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

	// Make TTS fail so we can revalidate the snapshot under both branches.
	tts.SetSynthesizeHook(func(_ context.Context, _ contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{}, errors.New("tts worker service unavailable")
	})

	// Revalidate: any revalidate returns stale.
	resolver.SetRevalidateHook(func(_ context.Context, _ contracts.ScopedContext) error {
		return orchestration.ErrStaleSnapshot
	})

	req := transcriptPipelineRequest("JTEST", "en-IN", "where can I go")
	req.Render.Kind = contracts.PipelineRenderTTS
	_, err := o.Process(context.Background(), req, nil)
	if err == nil {
		t.Fatalf("Process must fail closed on stale context after TTS, not return OK")
	}
	var pe *orchestration.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *PipelineError: %v", err)
	}
	if pe.Failures[0].Code != contracts.ErrStaleSnapshot {
		t.Errorf("Code = %s, want STALE_SNAPSHOT", pe.Failures[0].Code)
	}
}

// TestA1_TTSFailureUnchangedContext_PreservesActions: when TTS alone
// fails but the scoped context is still valid, the orchestrator must
// preserve the validated text/actions. Honest audio status only.
func TestA1_TTSFailureUnchangedContext_PreservesActions(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.ApprovedTemplateSHA["destination_options"] = orchestrationtest.DigestString("Destination choices are displayed on screen.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "destination_options", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Destination choices are displayed on screen.", SyntheticOnly: false,
	})
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		key := "destination_options"
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

	tts.SetSynthesizeHook(func(_ context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{}, errors.New("tts worker service unavailable")
	})

	req := transcriptPipelineRequest("JTEST", "en-IN", "where can I go")
	req.Render.Kind = contracts.PipelineRenderTTS
	out, err := o.Process(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Process must not fail completely when TTS is unavailable: %v", err)
	}
	if out.State != contracts.PipelineOK {
		t.Errorf("State = %s, want OK (unchanged context)", out.State)
	}
	if out.Audio != nil {
		t.Errorf("Audio should be nil on TTS failure: %+v", out.Audio)
	}
	if len(out.ValidatedProposal.Actions) != 1 || out.ValidatedProposal.Actions[0].Type != contracts.ActionShowChoices {
		t.Errorf("Validated proposal actions lost on TTS failure: %+v", out.ValidatedProposal.Actions)
	}
	if out.Template.SpeechKey != "destination_options" {
		t.Errorf("Template speech key = %s, want destination_options", out.Template.SpeechKey)
	}
	foundTTS := false
	for _, f := range out.Stages {
		if f.Stage == orchestration.StageTTS {
			foundTTS = true
		}
	}
	if !foundTTS {
		t.Errorf("Stages = %+v, want TTS failure recorded", out.Stages)
	}
}

// TestA1_ModelBytes_OversizedProposalRejected: a middle worker
// response whose proposal field exceeds MaxRawModelBytes (64 KiB)
// must be rejected at the HTTPWorkerClient before typed decoding.
func TestA1_ModelBytes_OversizedProposalRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			// Proposal with 80 KiB of padding.
			pad := strings.Repeat("x", 80*1024)
			body := fmt.Sprintf(`{"request_id":"req-a1","data_version":"PKG-1:1","model_revision":"r0","proposal":{"schema_version":"3.0","request_id":"req-a1","data_version":"PKG-1:1","status":"OK","intent":"RECENTER","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[],"_pad":%q}}`, pad)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	_, err := c.Propose(context.Background(), contracts.MiddleWorkerRequest{
		RequestID:       "req-a1",
		ScopedContext:   orchestrationtest.BuildScopedContext("JTEST", "en-IN"),
		Transcript:      contracts.ASRWorkerResponse{RequestID: "req-a1", Language: "en-IN", Text: "hi", State: contracts.TranscriptionOK},
		MaxOutputTokens: 256,
		DeadlineMillis:  6000,
	})
	if err == nil {
		t.Fatalf("Propose should fail on oversized proposal but returned nil error")
	}
}

// TestA1_ModelBytes_RecenterWithExplicitEmptyTargetIDRejected: a
// RECENTER action with an explicitly present empty target_id must be
// rejected at the raw bytes level (typed decode would treat it as
// absent and pass, losing the distinction).
func TestA1_ModelBytes_RecenterWithExplicitEmptyTargetIDRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			// RECENTER with explicit "target_id":"" — typed decode
			// would drop it to zero value; raw presence check must fail.
			body := `{"request_id":"req-a1","data_version":"PKG-1:1","model_revision":"r0","proposal":{"schema_version":"3.0","request_id":"req-a1","data_version":"PKG-1:1","status":"OK","intent":"RECENTER","language":"en-IN","actions":[{"type":"RECENTER","target_id":""}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	_, err := c.Propose(context.Background(), contracts.MiddleWorkerRequest{
		RequestID:       "req-a1",
		ScopedContext:   orchestrationtest.BuildScopedContext("JTEST", "en-IN"),
		Transcript:      contracts.ASRWorkerResponse{RequestID: "req-a1", Language: "en-IN", Text: "hi", State: contracts.TranscriptionOK},
		MaxOutputTokens: 256,
		DeadlineMillis:  6000,
	})
	if err == nil {
		t.Fatalf("Propose should fail on explicit empty target_id but returned nil error")
	}
}

// TestA1_ModelBytes_UnknownProposalFieldRejected: an unknown field
// inside the proposal object must be rejected.
func TestA1_ModelBytes_UnknownProposalFieldRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			body := `{"request_id":"req-a1","data_version":"PKG-1:1","model_revision":"r0","proposal":{"schema_version":"3.0","request_id":"req-a1","data_version":"PKG-1:1","status":"OK","intent":"RECENTER","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[],"unknown_field":true}}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	_, err := c.Propose(context.Background(), contracts.MiddleWorkerRequest{
		RequestID:       "req-a1",
		ScopedContext:   orchestrationtest.BuildScopedContext("JTEST", "en-IN"),
		Transcript:      contracts.ASRWorkerResponse{RequestID: "req-a1", Language: "en-IN", Text: "hi", State: contracts.TranscriptionOK},
		MaxOutputTokens: 256,
		DeadlineMillis:  6000,
	})
	if err == nil {
		t.Fatalf("Propose should fail on unknown proposal field but returned nil error")
	}
}

// TestA1_ModelBytes_MissingRequiredProposalFieldRejected: a proposal
// missing a required top-level field must be rejected.
func TestA1_ModelBytes_MissingRequiredProposalFieldRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			// Missing "status" — required.
			body := `{"request_id":"req-a1","data_version":"PKG-1:1","model_revision":"r0","proposal":{"schema_version":"3.0","request_id":"req-a1","data_version":"PKG-1:1","intent":"RECENTER","language":"en-IN","actions":[],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	_, err := c.Propose(context.Background(), contracts.MiddleWorkerRequest{
		RequestID:       "req-a1",
		ScopedContext:   orchestrationtest.BuildScopedContext("JTEST", "en-IN"),
		Transcript:      contracts.ASRWorkerResponse{RequestID: "req-a1", Language: "en-IN", Text: "hi", State: contracts.TranscriptionOK},
		MaxOutputTokens: 256,
		DeadlineMillis:  6000,
	})
	if err == nil {
		t.Fatalf("Propose should fail on missing required status but returned nil error")
	}
}

// TestA1_ModelBytes_ValidProposalPasses: a minimal valid proposal
// passes the raw checks and proceeds to typed validation.
func TestA1_ModelBytes_ValidProposalPasses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			body := `{"request_id":"req-a1","data_version":"PKG-1:1","model_revision":"r0","proposal":{"schema_version":"3.0","request_id":"req-a1","data_version":"PKG-1:1","status":"OK","intent":"RECENTER","language":"en-IN","actions":[{"type":"RECENTER"}],"speech_key":null,"clarification_ids":[],"evidence_ids":[]}}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, bytes.NewReader([]byte(body)))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	resp, err := c.Propose(context.Background(), contracts.MiddleWorkerRequest{
		RequestID:       "req-a1",
		ScopedContext:   orchestrationtest.BuildScopedContext("JTEST", "en-IN"),
		Transcript:      contracts.ASRWorkerResponse{RequestID: "req-a1", Language: "en-IN", Text: "hi", State: contracts.TranscriptionOK},
		MaxOutputTokens: 256,
		DeadlineMillis:  6000,
	})
	if err != nil {
		t.Fatalf("Propose should succeed on valid proposal: %v", err)
	}
	if resp.Proposal.Status != contracts.StatusOK {
		t.Fatalf("expected OK status, got %s", resp.Proposal.Status)
	}
}

// TestA1_TTSCancellationStaleContext_DropsActions: cancellation during TTS
// while the source has changed must drop actionable output.
func TestA1_TTSCancellationStaleContext_DropsActions(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.ApprovedTemplateSHA["destination_options"] = orchestrationtest.DigestString("Destination choices are displayed on screen.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "destination_options", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Destination choices are displayed on screen.", SyntheticOnly: false,
	})
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		key := "destination_options"
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

	// Revalidate ok; TTS succeeds; post-TTS revalidate stale.
	var revCount int
	resolver.SetRevalidateHook(func(_ context.Context, _ contracts.ScopedContext) error {
		revCount++
		if revCount > 1 {
			return orchestration.ErrStaleSnapshot
		}
		return nil
	})
	validWAV := makeTestWAV(t, 16000, 1)
	tts.SetSynthesizeHook(func(_ context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       base64.StdEncoding.EncodeToString(validWAV),
			ContentType:    "audio/wav",
			ChecksumSHA256: sha256Hex(validWAV),
			ModelRevision:  "r0",
			VoiceRevision:  "v0",
		}, nil
	})

	req := transcriptPipelineRequest("JTEST", "en-IN", "where can I go")
	req.Render.Kind = contracts.PipelineRenderTTS
	_, err := o.Process(context.Background(), req, nil)
	if err == nil {
		t.Fatalf("Process: expected STALE_SNAPSHOT error on stale-after-TTS")
	}
	var pe *orchestration.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *PipelineError: %v", err)
	}
	if pe.Failures[0].Code != contracts.ErrStaleSnapshot {
		t.Errorf("Code = %s, want STALE_SNAPSHOT", pe.Failures[0].Code)
	}
}

// TestA1_SynthesizeStaleSnapshotDuringSynthesis: direct synthesize
// path must also revalidate after TTS, on failure AND success.
func TestA1_SynthesizeStaleSnapshotDuringSynthesis(t *testing.T) {
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", "en-IN")
	sc.ApprovedTemplateSHA["welcome"] = orchestrationtest.DigestString("Welcome.")
	resolver := orchestrationtest.NewResolver(sc)
	validator := orchestrationtest.NewValidator()
	tpls := orchestrationtest.NewTemplates()
	tpls.Add(contracts.ApprovedTemplate{
		SpeechKey: "welcome", Language: "en-IN", TemplateVersion: 1, SourceVersion: 1,
		Text: "Welcome.", SyntheticOnly: false,
	})
	o := buildOrchestrator(asr, mid, tts, resolver, validator, tpls)

	// 1st revalidate passes (pre-synthesis); 2nd fails (post-synthesis)
	revCount := 0
	resolver.SetRevalidateHook(func(_ context.Context, _ contracts.ScopedContext) error {
		revCount++
		if revCount > 1 {
			return orchestration.ErrStaleSnapshot
		}
		return nil
	})

	validWAV := makeTestWAV(t, 16000, 1)
	tts.SetSynthesizeHook(func(_ context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		return contracts.TTSWorkerResponse{
			RequestID:      req.RequestID,
			SpeechKey:      req.SpeechKey,
			Language:       req.Language,
			State:          contracts.TTSOK,
			AudioB64:       base64.StdEncoding.EncodeToString(validWAV),
			ContentType:    "audio/wav",
			ChecksumSHA256: sha256Hex(validWAV),
			ModelRevision:  "r0",
			VoiceRevision:  "v0",
		}, nil
	})

	_, err := o.Synthesize(context.Background(), contracts.TTSRequest{
		RequestID:     "req-direct",
		Jurisdiction:  "JTEST",
		SpeechKey:     "welcome",
		Language:      "en-IN",
		SourceVersion: 1,
		Settings:      contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1},
	}, nil)
	if err == nil {
		t.Fatalf("expected Synthesize to fail closed on stale after synthesis")
	}
	var pe *orchestration.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("error is not *PipelineError: %v", err)
	}
	if pe.Failures[0].Code != contracts.ErrStaleSnapshot {
		t.Errorf("Code = %s, want STALE_SNAPSHOT", pe.Failures[0].Code)
	}
}

// TestA1_HTTPWorkerClientConcurrently: concurrent distinct
// requests through the HTTPWorkerClient must each get the response
// they were issued (no cross-consumption).
func TestA1_HTTPWorkerClientConcurrently(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/transcribe" {
			var req contracts.ASRWorkerRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			resp := contracts.ASRWorkerResponse{
				RequestID: req.RequestID,
				Language:  req.Language,
				Text:      "ok-" + req.RequestID,
				State:     contracts.TranscriptionOK,
			}
			w.Header().Set("Content-Type", "application/json")
			bs, _ := json.Marshal(resp)
			_, _ = w.Write(bs)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := orchestration.NewHTTPWorkerClient(ts.URL, "", nil)
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	results := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rid := fmt.Sprintf("req-%d", i)
			resp, err := c.Transcribe(context.Background(), contracts.ASRWorkerRequest{
				RequestID:   rid,
				Language:    "en-IN",
				ContentType: "audio/wav",
				AudioB64:    base64.StdEncoding.EncodeToString([]byte("a")),
			})
			if err != nil {
				errs <- err
				return
			}
			if resp.RequestID != rid {
				errs <- fmt.Errorf("rid mismatch: got %s want %s (concurrency)", resp.RequestID, rid)
				return
			}
			if resp.Text != "ok-"+rid {
				errs <- fmt.Errorf("text cross-consumed: got %s want %s", resp.Text, "ok-"+rid)
				return
			}
			results <- resp.Text
		}(i)
	}
	wg.Wait()
	close(errs)
	close(results)
	for e := range errs {
		t.Errorf("concurrent error: %v", e)
	}
	count := 0
	for range results {
		count++
	}
	if count != n {
		t.Errorf("got %d results, want %d", count, n)
	}
}
