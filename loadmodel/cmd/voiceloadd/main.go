// voiceloadd runs the voice-load harness against a configured set of
// dummy workers (started separately). The dummy workers must be listening
// on the addresses passed via env vars.
//
// Usage:
//   STHIRA_VOICE_ASR=http://127.0.0.1:9101 \
//   STHIRA_VOICE_MID=http://127.0.0.1:9102 \
//   STHIRA_VOICE_TTS=http://127.0.0.1:9103 \
//   voiceloadd --rate 20 --duration 30s --tts 0.5
//
// Output: a single JSON line on stdout that the harness captures.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sthira/backend/loadmodel/voice/voiceload"
)

func main() {
	rate := flag.Int("rate", 20, "utterances/s")
	duration := flag.Duration("duration", 30*time.Second, "run duration")
	ttsFrac := flag.Float64("tts", 0.5, "fraction of requests that include TTS")
	language := flag.String("language", "en-IN", "language header")
	jurisdiction := flag.String("jurisdiction", "KL", "jurisdiction")
	flag.Parse()

	cfg := voiceload.Config{
		Workers: voiceload.WorkerClients{
			ASR:    os.Getenv("STHIRA_VOICE_ASR"),
			Middle: os.Getenv("STHIRA_VOICE_MID"),
			TTS:    os.Getenv("STHIRA_VOICE_TTS"),
		},
		ArrivalRate:      *rate,
		Duration:         *duration,
		RenderTTSFraction: *ttsFrac,
		Language:         *language,
		Jurisdiction:     *jurisdiction,
		Logger:           slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	if cfg.Workers.ASR == "" || cfg.Workers.Middle == "" || cfg.Workers.TTS == "" {
		fmt.Fprintln(os.Stderr, "STHIRA_VOICE_ASR, STHIRA_VOICE_MID, STHIRA_VOICE_TTS must be set")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	res := voiceload.Run(ctx, cfg)
	_ = json.NewEncoder(os.Stdout).Encode(res)
}
