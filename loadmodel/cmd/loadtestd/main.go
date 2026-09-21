// loadtestd is the synthetic fixture server binary. It runs the same route
// set as the production /api/v3 boundary but with in-memory backends and
// switchable knobs. Used only by the P7 load harness.
//
// Usage:
//   loadtestd --addr 127.0.0.1:8080
//
// Knobs (env vars; the harness uses one process per scenario and may flip
// knobs mid-run via the /admin/* endpoints):
//   STHIRA_LOAD_CACHE_HIT    float [0..1], default 0.95
//   STHIRA_LOAD_QUEUE_DEPTH  int,         default 8
//   STHIRA_LOAD_SERVICE_ASR  duration,    default 1.5s
//   STHIRA_LOAD_SERVICE_MID  duration,    default 4s
//   STHIRA_LOAD_SERVICE_TTS  duration,    default 2s
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"sthira/backend/loadmodel/loadfixtures"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "bind address; :0 picks ephemeral")
	flag.Parse()

	cfg := loadfixtures.Config{
		Addr:           *addr,
		CacheHitRate:   envFloat("STHIRA_LOAD_CACHE_HIT", 0.95),
		OutageRate:     envFloat("STHIRA_LOAD_OUTAGE_RATE", 0.0),
		ManifestBytes:  4096,
		CardBytes:      16384,
		ResourceBytes:  8192,
		ServiceASR:     envDur("STHIRA_LOAD_SERVICE_ASR", 1500*time.Millisecond),
		ServiceMiddle:  envDur("STHIRA_LOAD_SERVICE_MID", 4*time.Second),
		ServiceTTS:     envDur("STHIRA_LOAD_SERVICE_TTS", 2*time.Second),
		QueueDepth:     envInt("STHIRA_LOAD_QUEUE_DEPTH", 8),
		WriteContention: 8,
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	srv := loadfixtures.New(cfg)
	if err := srv.Listen(); err != nil {
		fmt.Fprintln(os.Stderr, "loadtestd:", err)
		os.Exit(1)
	}
	fmt.Println("loadtestd listening on", srv.Addr())
	// Block forever; the harness sends SIGTERM.
	select {}
}

func envFloat(k string, def float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	var f float64
	_, err := fmt.Sscanf(v, "%f", &f)
	if err != nil {
		return def
	}
	return f
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	var i int
	_, err := fmt.Sscanf(v, "%d", &i)
	if err != nil {
		return def
	}
	return i
}

func envDur(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
