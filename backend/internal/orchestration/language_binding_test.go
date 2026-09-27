package orchestration_test

// Regressions for the response-language binding between the approved
// template, its language-bound digest, the TTS worker request and the
// returned audio metadata. Before the fix, stageTemplate rendered in
// proposal.Language while stageTTS labelled the text with req.Language
// (e.g. English text sent to TTS as hi-IN).

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"sthira/backend/internal/contracts"
	"sthira/backend/internal/orchestration"
	"sthira/backend/internal/orchestration/orchestrationtest"
)

const (
	langEN   = "en-IN"
	langHI   = "hi-IN"
	textEN   = "Destination choices are displayed on screen."
	textHI   = "गंतव्य विकल्प स्क्रीन पर दिखाए गए हैं।"
	destKey  = "destination_options"
	langKey  = "language_changed"
	textLHI  = "भाषा हिंदी में बदल दी गई है।"
	textLEN  = "Language changed."
	facility = "FAC-DEMO-1"
)

type langRig struct {
	o        *orchestration.Orchestrator
	mid, tts *orchestrationtest.Worker
	calls    []contracts.TTSWorkerRequest
	wav      []byte
	sc       contracts.ScopedContext
	// ttsLanguageOverride, when set, makes the fake worker attest a
	// different language than requested.
	ttsLanguageOverride string
	ttsFail             bool
}

func newLangRig(t *testing.T) *langRig {
	t.Helper()
	asr, mid, tts := orchestrationtest.NewWorker(), orchestrationtest.NewWorker(), orchestrationtest.NewWorker()
	sc := orchestrationtest.BuildScopedContext("JTEST", langEN)
	sc.AllowedLanguages = []string{langEN, langHI}
	sc.TemplateKeys = append(sc.TemplateKeys, langKey)
	sc.ApprovedSpeechKeys[destKey] = []string{langEN, langHI}
	sc.ApprovedSpeechKeys[langKey] = []string{langEN, langHI}
	sc.ApprovedTemplateSHA[contracts.TemplateDigestKey(destKey, langHI)] = orchestrationtest.DigestString(textHI)
	sc.ApprovedTemplateSHA[contracts.TemplateDigestKey(langKey, langHI)] = orchestrationtest.DigestString(textLHI)
	sc.ApprovedTemplateSHA[contracts.TemplateDigestKey(langKey, langEN)] = orchestrationtest.DigestString(textLEN)
	tpls := orchestrationtest.NewTemplates()
	for _, tp := range []contracts.ApprovedTemplate{
		{SpeechKey: destKey, Language: langEN, Text: textEN},
		{SpeechKey: destKey, Language: langHI, Text: textHI},
		{SpeechKey: langKey, Language: langEN, Text: textLEN},
		{SpeechKey: langKey, Language: langHI, Text: textLHI},
	} {
		tp.TemplateVersion, tp.SourceVersion = 1, 1
		tpls.Add(tp)
	}
	r := &langRig{mid: mid, tts: tts, wav: makeTestWAV(t, 16000, 0.1), sc: sc}
	r.o = buildOrchestrator(asr, mid, tts, orchestrationtest.NewResolver(sc), orchestrationtest.NewValidator(), tpls)
	tts.SetSynthesizeHook(func(_ context.Context, req contracts.TTSWorkerRequest) (contracts.TTSWorkerResponse, error) {
		r.calls = append(r.calls, req)
		if r.ttsFail {
			return contracts.TTSWorkerResponse{}, errors.New("tts worker down")
		}
		lang := req.Language
		if r.ttsLanguageOverride != "" {
			lang = r.ttsLanguageOverride
		}
		return contracts.TTSWorkerResponse{
			RequestID: req.RequestID, SpeechKey: req.SpeechKey, Language: lang,
			State: contracts.TTSOK, AudioB64: base64.StdEncoding.EncodeToString(r.wav),
			ContentType: "audio/wav", ChecksumSHA256: sha256Hex(r.wav),
			ModelRevision: "fake", VoiceRevision: "fake",
			Settings: contracts.TTSSynthesisSettings{SampleRate: 16000, BitDepth: 16, Channels: 1},
		}, nil
	})
	return r
}

func (r *langRig) propose(lang string, key *string, intent contracts.Intent, actions []contracts.Action) {
	r.mid.SetProposeHook(func(_ context.Context, req contracts.MiddleWorkerRequest) (contracts.MiddleWorkerResponse, error) {
		return contracts.MiddleWorkerResponse{
			RequestID: req.RequestID,
			Proposal: contracts.ModelOutput{
				SchemaVersion: contracts.ModelSchemaVersion, RequestID: req.RequestID,
				DataVersion: req.ScopedContext.DataVersion, Status: contracts.StatusOK,
				Intent: &intent, Language: lang, Actions: actions, SpeechKey: key,
				ClarificationIDs: []string{}, EvidenceIDs: []string{},
			},
			ModelRevision: "fake",
		}, nil
	})
}

func (r *langRig) run(t *testing.T, reqLang string) orchestration.PipelineOutput {
	t.Helper()
	req := transcriptPipelineRequest("JTEST", reqLang, "where can I go")
	req.Render.Kind = contracts.PipelineRenderTTS
	out, err := r.o.Process(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	return out
}

func hasStage(out orchestration.PipelineOutput, s orchestration.Stage) bool {
	for _, f := range out.Stages {
		if f.Stage == s {
			return true
		}
	}
	return false
}

func choices() []contracts.Action {
	return []contracts.Action{{Type: contracts.ActionShowChoices, TargetIDs: []string{facility}}}
}

func TestLanguageBinding_SameLanguageSynthesizes(t *testing.T) {
	r := newLangRig(t)
	k := destKey
	r.propose(langHI, &k, contracts.IntentListDestinations, choices())
	out := r.run(t, langHI)
	if len(out.Stages) != 0 || len(r.calls) != 1 || out.Audio == nil {
		t.Fatalf("stages=%v calls=%d audio=%v", out.Stages, len(r.calls), out.Audio)
	}
	c := r.calls[0]
	if c.Language != langHI || c.Text != textHI || c.TemplateSHA256 != orchestrationtest.DigestString(textHI) {
		t.Fatalf("TTS request not bound to hi-IN approved text: %+v", c)
	}
	if out.Template.Text != textHI || out.Audio.Language != langHI {
		t.Fatalf("template/audio language disagree: text=%q audio=%s", out.Template.Text, out.Audio.Language)
	}
}

func TestLanguageBinding_CrossLanguageMismatchNeverSynthesizes(t *testing.T) {
	r := newLangRig(t)
	k := destKey
	// Hindi request, English proposal, no language switch.
	r.propose(langEN, &k, contracts.IntentListDestinations, choices())
	out := r.run(t, langHI)
	if len(r.calls) != 0 {
		t.Fatalf("TTS must not be called for mismatched language; got %+v", r.calls)
	}
	if out.Audio != nil || out.Template.Text != "" {
		t.Fatalf("no text/audio may be emitted: text=%q audio=%v", out.Template.Text, out.Audio)
	}
	if !hasStage(out, orchestration.StageTemplate) {
		t.Fatalf("expected template stage failure, got %v", out.Stages)
	}
}

func TestLanguageBinding_WorkerMislabelRejected(t *testing.T) {
	r := newLangRig(t)
	k := destKey
	r.ttsLanguageOverride = langEN
	r.propose(langHI, &k, contracts.IntentListDestinations, choices())
	out := r.run(t, langHI)
	if out.Audio != nil || !hasStage(out, orchestration.StageTTS) {
		t.Fatalf("mislabelled worker audio must be rejected: audio=%v stages=%v", out.Audio, out.Stages)
	}
}

func TestLanguageBinding_SetLanguageSwitchSpeaksNewLanguage(t *testing.T) {
	r := newLangRig(t)
	k := langKey
	// English request asks to switch to Hindi; confirmation is in Hindi.
	r.propose(langHI, &k, contracts.IntentChangeLanguage, []contracts.Action{{Type: contracts.ActionSetLanguage, Language: langHI}})
	out := r.run(t, langEN)
	if len(out.Stages) != 0 || len(r.calls) != 1 || out.Audio == nil {
		t.Fatalf("stages=%v calls=%d audio=%v", out.Stages, len(r.calls), out.Audio)
	}
	c := r.calls[0]
	if c.Language != langHI || c.Text != textLHI || c.TemplateSHA256 != orchestrationtest.DigestString(textLHI) || out.Audio.Language != langHI {
		t.Fatalf("switch confirmation not bound to hi-IN: req=%+v audio=%s", c, out.Audio.Language)
	}

	// A SET_LANGUAGE to a different language does not license speech in
	// a third (or the request) language mismatch.
	r2 := newLangRig(t)
	r2.propose(langHI, &k, contracts.IntentChangeLanguage, []contracts.Action{{Type: contracts.ActionSetLanguage, Language: langEN}})
	out2 := r2.run(t, langEN)
	if len(r2.calls) != 0 || out2.Audio != nil {
		t.Fatalf("SET_LANGUAGE en-IN must not license hi-IN speech: calls=%d", len(r2.calls))
	}
}

func TestLanguageBinding_SilentActionNoTTS(t *testing.T) {
	r := newLangRig(t)
	r.ttsFail = true
	r.propose(langEN, nil, contracts.IntentZoom, []contracts.Action{{Type: contracts.ActionZoom, Direction: "IN", Steps: 1}})
	// Even with a language mismatch, a silent action has no speech to bind.
	out := r.run(t, langHI)
	if len(r.calls) != 0 || out.Audio != nil || len(out.Stages) != 0 {
		t.Fatalf("silent action touched TTS: calls=%d stages=%v", len(r.calls), out.Stages)
	}
	if out.State != contracts.PipelineOK || len(out.ValidatedProposal.Actions) != 1 {
		t.Fatalf("silent action not preserved: state=%s", out.State)
	}
}

func TestLanguageBinding_TTSFailureNoFabricatedAudio(t *testing.T) {
	r := newLangRig(t)
	r.ttsFail = true
	k := destKey
	r.propose(langHI, &k, contracts.IntentListDestinations, choices())
	out := r.run(t, langHI)
	if out.Audio != nil {
		t.Fatalf("audio fabricated on TTS failure")
	}
	if !hasStage(out, orchestration.StageTTS) || out.State != contracts.PipelineOK {
		t.Fatalf("want OK with tts stage failure, got state=%s stages=%v", out.State, out.Stages)
	}
	if out.Template.Text != textHI || len(out.ValidatedProposal.Actions) != 1 {
		t.Fatalf("validated text/actions must survive optional TTS failure")
	}
}
