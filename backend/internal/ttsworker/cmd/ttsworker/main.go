// Command ttsworker runs the private Indic Parler-TTS worker service.
// It wraps the real Python adapter subprocess and exposes the private HTTP protocol.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"sthira/backend/internal/ttsworker"
	"sthira/backend/internal/ttsworker/templates"
)

func main() {
	addr := os.Getenv("STHIRA_TTS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8003"
	}
	tok := os.Getenv("STHIRA_TTS_TOKEN")

	pyCmd := os.Getenv("STHIRA_TTS_PYTHON")
	if pyCmd == "" {
		pyCmd = "python3"
	}
	module := os.Getenv("STHIRA_TTS_ADAPTER")
	if module == "" {
		module = "sthira_v2.speech_tts_adapter"
	}
	workdir := os.Getenv("STHIRA_TTS_WORKDIR")

	queueDepth := 8
	if q := os.Getenv("STHIRA_TTS_QUEUE_DEPTH"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			queueDepth = v
		}
	}
	maxInflight := 2
	if m := os.Getenv("STHIRA_TTS_MAX_INFLIGHT"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			maxInflight = v
		}
	}
	sourceVersion := 1
	if s := os.Getenv("STHIRA_TTS_SOURCE_VERSION"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			sourceVersion = v
		}
	}

	catalog := templates.NewCatalog()
	defaultTemplates := []*templates.Template{
		{
			Key:       "destination_options",
			Status:    templates.StatusApproved,
			Version:   1,
			Languages: []string{"en-IN", "hi-IN", "ml-IN"},
			Translations: map[string]string{
				"en-IN": "Destination choices are displayed on screen.",
				"hi-IN": "गंतव्य विकल्प स्क्रीन पर प्रदर्शित हैं।",
				"ml-IN": "ലക്ഷ്യസ്ഥാന ഓപ്ഷനുകൾ സ്ക്രീനിൽ കാണിച്ചിരിക്കുന്നു.",
			},
		},
		{
			Key:       "clarify_place",
			Status:    templates.StatusApproved,
			Version:   1,
			Languages: []string{"en-IN", "hi-IN", "ml-IN"},
			Translations: map[string]string{
				"en-IN": "Please clarify the location.",
				"hi-IN": "कृपया स्थान स्पष्ट करें।",
				"ml-IN": "ദയവായി സ്ഥലം വ്യക്തമാക്കുക.",
			},
		},
		{
			Key:       "verified_route_unavailable",
			Status:    templates.StatusApproved,
			Version:   1,
			Languages: []string{"en-IN", "hi-IN", "ml-IN"},
			Translations: map[string]string{
				"en-IN": "Verified route is currently unavailable.",
				"hi-IN": "सत्यापित मार्ग वर्तमान में अनुपलब्ध है।",
				"ml-IN": "സ്ഥിരീകരിച്ച റൂട്ട് നിലവിൽ ലഭ്യമല്ല.",
			},
		},
		{
			Key:       "welcome",
			Status:    templates.StatusApproved,
			Version:   1,
			Languages: []string{"en-IN", "hi-IN", "ml-IN"},
			Translations: map[string]string{
				"en-IN": "Welcome to Sthira emergency guidance.",
				"hi-IN": "स्थिरा आपातकालीन मार्गदर्शन में आपका स्वागत है।",
				"ml-IN": "സ്ഥിര അടിയന്തര മാർഗ്ഗനിർദ്ദേശത്തിലേക്ക് സ്വാഗതം.",
			},
		},
	}
	for _, tpl := range defaultTemplates {
		if err := catalog.Register(tpl); err != nil {
			log.Fatalf("failed to register template %s: %v", tpl.Key, err)
		}
	}

	renderer := templates.NewRenderer(catalog)
	clock := ttsworker.NewStandaloneSourceVersionClock(sourceVersion)
	codec := ttsworker.NewCodec(64*1024*1024, time.Hour, clock)

	rt := ttsworker.NewAdapterSubprocessRuntime(ttsworker.AdapterSubprocessConfig{
		Cmd:     pyCmd,
		Module:  module,
		Workdir: workdir,
	})

	if err := rt.LoadModel(); err != nil {
		log.Fatalf("failed to load tts model (failing closed): %v", err)
	}

	var voiceNames []string
	for _, v := range rt.Voices() {
		voiceNames = append(voiceNames, v.Name)
	}
	langs := rt.Languages()
	if len(langs) == 0 {
		langs = []string{"hi-IN", "ml-IN", "en-IN"}
	}
	inv := ttsworker.Inventory{
		Parler: ttsworker.ParlerTTS{
			ModelID:        "ai4bharat/indic-parler-tts",
			Revision:       rt.Revision(),
			License:        "Apache-2.0",
			PretrainedFile: "model.safetensors",
			Runtime:        "transformers-4.x",
			Hardware:       "cpu",
			Voices:         voiceNames,
		},
		SupportedLanguages: langs,
	}

	worker, err := ttsworker.New(ttsworker.Config{
		Inventory:   inv,
		Runtime:     rt,
		Catalog:     catalog,
		Renderer:    renderer,
		Cache:       codec,
		Clock:       clock,
		QueueDepth:  queueDepth,
		MaxInFlight: maxInflight,
	})
	if err != nil {
		log.Fatalf("failed to create tts worker: %v", err)
	}

	srv, err := ttsworker.NewServer(ttsworker.ServerConfig{
		Address:     addr,
		BearerToken: tok,
		Worker:      worker,
	})
	if err != nil {
		log.Fatalf("failed to create tts server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Cancel(shutdownCtx)
		_ = worker.Shutdown()
		_ = rt.Close()
	}()

	log.Printf("ttsworker listening on %s (private)", addr)
	if err := srv.Start(ctx, addr); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		log.Fatalf("ttsworker server exited: %v", err)
	}
}
