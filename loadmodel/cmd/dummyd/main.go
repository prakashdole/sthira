// dummyd runs one of the three private P6 worker protocols (ASR / middle /
// TTS) behind a fixed-latency, deterministic synthetic worker. It is used
// only by the P7 load harness. Throughput measured against this dummy is
// NOT GPU throughput.
//
// Usage:
//
//	dummyd --kind asr     --addr 127.0.0.1:9101
//	dummyd --kind middle  --addr 127.0.0.1:9102
//	dummyd --kind tts     --addr 127.0.0.1:9103
//
// Routes exposed (always; the kind controls only the default service time):
//
//	/health                       -> {"ready":bool,"warm":bool}
//	/transcribe                   -> ASR
//	/v1/chat/completions          -> middle
//	/synthesize                   -> TTS
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sthira/backend/loadmodel/loadfixtures/dummyd"
)

func main() {
	kind := flag.String("kind", "asr", "worker kind: asr | middle | tts")
	addr := flag.String("addr", "127.0.0.1:0", "bind address; :0 picks ephemeral")
	serviceStr := flag.String("service", "", "service time (e.g. 1.5s); default per kind")
	errRate := flag.Float64("error-rate", 0.0, "probability of MODEL_UNAVAILABLE response [0..1]")
	concurrency := flag.Int("concurrency", 32, "max in-flight requests")
	flag.Parse()

	svc := time.Duration(0)
	if *serviceStr != "" {
		d, err := time.ParseDuration(*serviceStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad service duration:", err)
			os.Exit(1)
		}
		svc = d
	}

	cfg := dummyd.DummyWorkerConfig{
		Addr:        *addr,
		Kind:        *kind,
		ServiceTime: svc,
		ErrorRate:   *errRate,
		Concurrency: *concurrency,
		Logger:      slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}
	w, err := dummyd.StartDummyWorker(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dummyd:", err)
		os.Exit(1)
	}
	fmt.Println("dummyd listening on", w.URL(), "kind="+*kind)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = w.Shutdown(shutdownCtx)
}
