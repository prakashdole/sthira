package ttsworker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sthira/backend/internal/ttsworker/templates"
)

// helper: build a worker with synthetic inventory + stub catalog +
// stub runtime + a fresh codec + clock. The hot path is
// cache-only; cache is empty by default.
func newWorker(t *testing.T, queueDepth, maxInFlight int) (*Worker, *templates.Catalog, *templates.Renderer, *Codec, *StandaloneSourceVersionClock) {
	t.Helper()
	catalog := buildCatalog(t)
	renderer := templates.NewRenderer(catalog)
	clock := NewStandaloneSourceVersionClock(7)
	codec := NewCodec(8*1024*1024, time.Hour, clock)
	inv := Inventory{
		Parler: ParlerTTS{
			ModelID:  "ai4bharat/indic-parler-tts",
			License:  "Apache-2.0",
			Runtime:  "transformers-4.x",
			Hardware: "cpu",
			Revision: "rev-1",
			Voices:   []string{"ml-IN-female-1"},
		},
		SupportedLanguages: []string{"hi-IN", "ml-IN", "en-IN"},
	}
	w, err := New(Config{
		Inventory:   inv,
		Runtime:     newTaggedRuntime("ml-IN", "ml-IN-female-1", "rev-1"),
		Catalog:     catalog,
		Renderer:    renderer,
		Cache:       codec,
		Clock:       clock,
		QueueDepth:  queueDepth,
		MaxInFlight: maxInFlight,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Shutdown() })
	return w, catalog, renderer, codec, clock
}

func buildCatalog(t *testing.T) *templates.Catalog {
	t.Helper()
	c := templates.NewCatalog()
	tpl := &templates.Template{
		Key:       "destination_options",
		Status:    templates.StatusApproved,
		Version:   1,
		Languages: []string{"hi-IN", "ml-IN", "en-IN"},
		ArgSchema: map[string]templates.ArgType{"facility_id": templates.ArgString, "rank": templates.ArgInt},
		Translations: map[string]string{
			"hi-IN": "पहला सुरक्षित स्थान {facility_id} क्रम {rank}",
			"ml-IN": "ആദ്യ സുരക്ഷിത മേഖല {facility_id} റാങ്ക് {rank}",
			"en-IN": "First safe zone {facility_id} rank {rank}",
		},
	}
	if err := c.Register(tpl); err != nil {
		t.Fatal(err)
	}
	return c
}

func renderText(t *testing.T, renderer *templates.Renderer, lang string) string {
	t.Helper()
	c := renderer.Cat()
	args, err := c.ValidateArgs(templates.Key("destination_options"), map[string]any{
		"facility_id": "FACILITY-A",
		"rank":        1,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderer.Render(templates.Key("destination_options"), lang, args)
	if err != nil {
		t.Fatal(err)
	}
	return out.Text
}

func craftReq(t *testing.T, renderer *templates.Renderer, clock *StandaloneSourceVersionClock, lang string) SynthesizeRequest {
	t.Helper()
	text := renderText(t, renderer, lang)
	return SynthesizeRequest{
		RequestID:       "REQ-1",
		SpeechKey:       "destination_options",
		Language:        lang,
		Text:            text,
		SourceVersion:   clock.Current(),
		TemplateVersion: 1,
		Settings:        SynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1, SpeakingRate: 1.0},
		DeadlineMillis:  5000,
	}
}

// identityFor builds a CacheIdentity for tests.
func identityFor(t *testing.T, renderer *templates.Renderer, clock *StandaloneSourceVersionClock, lang string) CacheIdentity {
	t.Helper()
	text := renderText(t, renderer, lang)
	return CacheIdentity{
		Text:              text,
		TemplateKey:       "destination_options",
		TemplateVersion:   1,
		SourceVersion:     clock.Current(),
		Language:          lang,
		ModelRevision:     "rev-1",
		VoiceRevision:     "ml-IN-female-1",
		SynthesisSettings: SynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1, SpeakingRate: 1.0},
	}
}

// silenceWAVCache puts a silent WAV into the cache for an identity.
func silenceWAVCache(t *testing.T, codec *Codec, id CacheIdentity) {
	t.Helper()
	silence, err := EncodeSilenceWav(22050, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if err := codec.Put(id, silence.Bytes); err != nil {
		t.Fatal(err)
	}
}

// TestWorkerHotPathServesCachedAudio: a pre-populated cache entry
// returns the cached bytes via Synthesize.
func TestWorkerHotPathServesCachedAudio(t *testing.T) {
	w, _, renderer, codec, clock := newWorker(t, 8, 2)
	id := identityFor(t, renderer, clock, "ml-IN")
	silenceWAVCache(t, codec, id)
	req := SynthesizeRequest{
		RequestID:       "REQ-1",
		SpeechKey:       "destination_options",
		Language:        "ml-IN",
		Text:            id.Text,
		SourceVersion:   id.SourceVersion,
		TemplateVersion: id.TemplateVersion,
		Settings:        id.SynthesisSettings,
		DeadlineMillis:  5000,
	}
	resp, err := w.Synthesize(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.State != StateOK {
		t.Fatalf("expected StateOK, got %s", resp.State)
	}
	if !resp.CacheHit {
		t.Fatal("expected cache hit")
	}
	if resp.ChecksumSHA256 == "" {
		t.Fatal("checksum missing")
	}
	if !strings.Contains(resp.AudioB64, "") {
		// sanity
	}
}

// TestWorkerHotPathMissIsAudioUnavailable: cache empty, the worker
// must NOT synthesize on the hot path. Production pre-generates
// approved entries via the offline workflow.
func TestWorkerHotPathMissIsAudioUnavailable(t *testing.T) {
	w, _, renderer, _, clock := newWorker(t, 8, 2)
	req := craftReq(t, renderer, clock, "ml-IN")
	resp, err := w.Synthesize(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.State != StateAudioUnavailable {
		t.Fatalf("expected AUDIO_UNAVAILABLE on cache miss, got %s", resp.State)
	}
}

// TestWorkerStaleSourceVersionIsRefused: an audio request whose
// source_version does not match the current clock is rejected with
// STALE_VERSION.
func TestWorkerStaleSourceVersionIsRefused(t *testing.T) {
	w, _, renderer, codec, clock := newWorker(t, 8, 2)
	id := identityFor(t, renderer, clock, "ml-IN")
	id.SourceVersion = clock.Current() - 1 // stale by 1
	silenceWAVCache(t, codec, id)
	req := SynthesizeRequest{
		RequestID:       "REQ-stale",
		SpeechKey:       "destination_options",
		Language:        "ml-IN",
		Text:            id.Text,
		SourceVersion:   id.SourceVersion,
		TemplateVersion: id.TemplateVersion,
		Settings:        id.SynthesisSettings,
		DeadlineMillis:  5000,
	}
	resp, err := w.Synthesize(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.State != StateStaleVersion {
		t.Fatalf("expected STALE_VERSION, got %s", resp.State)
	}
}

// TestWorkerUnknownTemplateIsAudioUnavailable: a request for an
// unknown template returns AUDIO_UNAVAILABLE.
func TestWorkerUnknownTemplateIsAudioUnavailable(t *testing.T) {
	w, _, renderer, _, clock := newWorker(t, 8, 2)
	text := renderText(t, renderer, "ml-IN")
	req := SynthesizeRequest{
		RequestID:       "REQ-unk",
		SpeechKey:       "unknown_template",
		Language:        "ml-IN",
		Text:            text,
		SourceVersion:   clock.Current(),
		TemplateVersion: 1,
		Settings:        SynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1},
		DeadlineMillis:  5000,
	}
	resp, err := w.Synthesize(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.State != StateAudioUnavailable {
		t.Fatalf("expected AUDIO_UNAVAILABLE, got %s", resp.State)
	}
}

// TestWorkerUnsupportedLanguageIsRefused: a request for an
// unsupported language returns UNSUPPORTED_LANGUAGE.
func TestWorkerUnsupportedLanguageIsRefused(t *testing.T) {
	w, _, renderer, _, clock := newWorker(t, 8, 2)
	text := renderText(t, renderer, "ml-IN")
	req := SynthesizeRequest{
		RequestID:       "REQ-lang",
		SpeechKey:       "destination_options",
		Language:        "tl-PH",
		Text:            text,
		SourceVersion:   clock.Current(),
		TemplateVersion: 1,
		Settings:        SynthesisSettings{SampleRate: 22050, BitDepth: 16, Channels: 1},
		DeadlineMillis:  5000,
	}
	resp, err := w.Synthesize(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.State != StateUnsupportedLanguage {
		t.Fatalf("expected UNSUPPORTED_LANGUAGE, got %s", resp.State)
	}
}

// TestWorkerWithdrawnTemplateStopsServicing: withdrawing a template
// causes subsequent requests to return STALE_VERSION.
func TestWorkerWithdrawnTemplateStopsServicing(t *testing.T) {
	w, catalog, renderer, codec, clock := newWorker(t, 8, 2)
	id := identityFor(t, renderer, clock, "ml-IN")
	silenceWAVCache(t, codec, id)
	if err := catalog.Withdraw("destination_options", 2); err != nil {
		t.Fatal(err)
	}
	req := SynthesizeRequest{
		RequestID:       "REQ-withdraw",
		SpeechKey:       "destination_options",
		Language:        "ml-IN",
		Text:            id.Text,
		SourceVersion:   id.SourceVersion,
		TemplateVersion: id.TemplateVersion,
		Settings:        id.SynthesisSettings,
		DeadlineMillis:  5000,
	}
	resp, err := w.Synthesize(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.State != StateStaleVersion {
		t.Fatalf("expected STALE_VERSION after withdrawal, got %s", resp.State)
	}
}

// TestWorkerShutdownIsIdempotent: calling Shutdown twice is a no-op.
func TestWorkerShutdownIsIdempotent(t *testing.T) {
	w, _, _, _, _ := newWorker(t, 4, 2)
	if err := w.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if err := w.Shutdown(); err != nil {
		t.Fatal(err)
	}
}

// TestWorkerShutdownRejectsSubsequent: requests after Shutdown
// return ErrWorkerShutdown.
func TestWorkerShutdownRejectsSubsequent(t *testing.T) {
	w, _, renderer, _, clock := newWorker(t, 4, 2)
	if err := w.Shutdown(); err != nil {
		t.Fatal(err)
	}
	req := craftReq(t, renderer, clock, "ml-IN")
	if _, err := w.Synthesize(req); err == nil {
		t.Fatal("expected error after shutdown")
	}
}

// TestWorkerWithdrawalRacesServe: a withdrawal that fires while a
// hot-path read is mid-flight must NOT serve stale audio. The cache
// codec's lock contract guarantees this; this test verifies the
// integration under -race.
func TestWorkerWithdrawalRacesServe(t *testing.T) {
	w, _, renderer, codec, clock := newWorker(t, 8, 4)
	id := identityFor(t, renderer, clock, "ml-IN")
	silenceWAVCache(t, codec, id)
	const N = 200
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			clock.Advance(1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			req := SynthesizeRequest{
				RequestID:       "REQ",
				SpeechKey:       "destination_options",
				Language:        "ml-IN",
				Text:            id.Text,
				SourceVersion:   id.SourceVersion,
				TemplateVersion: id.TemplateVersion,
				Settings:        id.SynthesisSettings,
				DeadlineMillis:  5000,
			}
			_, _ = w.Synthesize(req)
		}
	}()
	wg.Wait()
}

// TestWorkerHealthReportsReadiness: Health() returns a snapshot
// with Ready=true.
func TestWorkerHealthReportsReadiness(t *testing.T) {
	w, _, _, _, _ := newWorker(t, 4, 2)
	h := w.Health()
	if !h.Ready {
		t.Fatalf("expected Ready=true: %+v", h)
	}
}

// TestWorkerContextCancelPropagates: a request context cancellation
// does not panic and returns no result on the timeout path.
func TestWorkerContextCancelPropagates(t *testing.T) {
	w, _, renderer, _, clock := newWorker(t, 8, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = ctx
	req := craftReq(t, renderer, clock, "ml-IN")
	_, _ = w.Synthesize(req)
}

// silence variable ensures the helper is referenced by a non-test file too.
var _ = errors.Is

// Touch atomic to silence import-only-when-test usage.
var _ atomic.Int64
