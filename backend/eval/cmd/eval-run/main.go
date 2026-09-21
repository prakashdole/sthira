// Command eval-run runs the P6 evaluation harness against a corpus.
//
// Usage (deterministic harness self-check):
//
//	go run ./cmd/eval-run -suite ./cases/synthetic
//
// Usage (real run with synthetic-only filter):
//
//	go run ./cmd/eval-run -real -suite ./cases/synthetic \
//	    -asr http://localhost:7101 \
//	    -middle http://localhost:7201 \
//	    -tts http://localhost:7301
//
// Usage (real run with synthetic + consented manifest):
//
//	go run ./cmd/eval-run -real -suite ./cases/synthetic \
//	    -real-manifest ./outside-repo/manifest.json \
//	    -asr http://localhost:7101 -middle http://localhost:7201 -tts http://localhost:7301
//
// Flags:
//
//	-suite <path>           corpus directory or single file
//	-real                   opt-in real provider run
//	-real-manifest <path>   JSON file mapping CONSENTED case IDs to outside-repo audio
//	-split <EVAL|DEV|TRAIN> restrict to one split (default EVAL)
//	-asr <url>              ASR worker URL (real)
//	-middle <url>           Middle worker URL (real)
//	-tts <url>              TTS worker URL (real)
//	-bearer <token>         bearer token (real)
//	-budget-ms <int>        default per-case latency budget
//	-warm <int>             warm-up dispatches per (language,category)
//	-filter <kv-list>       comma-list key=value filters
//	-md-out <path>          write Markdown report to file
//	-json-out <path>        write JSON report to file
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sthira/backend/eval/corpus"
	"sthira/backend/eval/provider"
	"sthira/backend/eval/report"
	"sthira/backend/eval/runner"
)

// RealManifest is the JSON file that maps case IDs to on-disk audio.
// Lives outside the repo to keep personal audio out of Git.
type RealManifest struct {
	Cases []RealCaseEntry `json:"cases"`
}

// RealCaseEntry is one row of the manifest. The runner uses AudioDigest
// and AudioPath to dispatch real recorded audio.
type RealCaseEntry struct {
	CaseID      string            `json:"case_id"`
	AudioPath   string            `json:"audio_path"`
	AudioDigest string            `json:"audio_digest"`
	Lang        string            `json:"language"`
	Category    string            `json:"category"`
	Provenance  corpus.Provenance `json:"provenance"`
	Expected    corpus.Expected   `json:"expected"`
}

func main() {
	var (
		suitePath    = flag.String("suite", "", "path to corpus directory or file")
		realFlag     = flag.Bool("real", false, "enable real provider run")
		realManifest = flag.String("real-manifest", "", "path to consented-case manifest (outside-repo)")
		splitFlag    = flag.String("split", "EVAL", "restrict to one split")
		asrURL       = flag.String("asr", "", "ASR URL (real)")
		midURL       = flag.String("middle", "", "Middle URL (real)")
		ttsURL       = flag.String("tts", "", "TTS URL (real)")
		bearer       = flag.String("bearer", "", "auth bearer (real)")
		budgetMS     = flag.Int("budget-ms", 1500, "default per-case latency budget ms")
		warmRuns     = flag.Int("warm", 1, "warm-up dispatches per (language,category)")
		filter       = flag.String("filter", "", "comma-list key=value filters")
		outMD        = flag.String("md-out", "", "write Markdown report to file")
		outJSON      = flag.String("json-out", "", "write JSON report to file")
	)
	flag.Parse()

	if *suitePath == "" {
		fmt.Fprintln(os.Stderr, "missing -suite")
		os.Exit(2)
	}

	cfg := runner.DefaultConfig()
	cfg.WarmRuns = *warmRuns
	cfg.BudgetMS = *budgetMS
	cfg.Split = *splitFlag
	cfg.Filter = parseFilter(*filter)
	cfg.Mode = runner.ModeSynthetic
	if *realFlag {
		cfg.Mode = runner.ModeReal
	}

	if cfg.Mode == runner.ModeReal {
		hp := provider.NewHTTP(provider.HTTPConfig{
			ASRURL:     *asrURL,
			MiddleURL:  *midURL,
			TTSURL:     *ttsURL,
			AuthBearer: *bearer,
			Timeout:    5 * time.Second,
		})
		cfg.Provider = hp
	}

	synthetic, err := loadSuite(*suitePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load suite:", err)
		os.Exit(2)
	}
	suite := synthetic
	if *realManifest != "" {
		consented, err := loadManifest(*realManifest)
		if err != nil {
			fmt.Fprintln(os.Stderr, "load manifest:", err)
			os.Exit(2)
		}
		suite.Cases = append(suite.Cases, consented...)
	}

	r := runner.New(cfg)
	results, err := r.Run(context.Background(), suite)
	if err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(2)
	}
	rep := report.Build(results)

	if *outJSON != "" {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err == nil {
			_ = os.WriteFile(*outJSON, b, 0o644)
		}
	}
	if *outMD != "" {
		f, err := os.Create(*outMD)
		if err == nil {
			rep.WriteMarkdown(f)
			_ = f.Close()
		}
	}
	rep.WriteMarkdown(os.Stdout)
}

// loadSuite loads either a single JSON file or every *.json in a
// directory, treating each as a synthetic fixture.
func loadSuite(path string) (corpus.Suite, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return corpus.Suite{}, err
	}
	if !fi.IsDir() {
		return corpus.Load([]corpus.SourceLoc{
			{Path: path, Class: corpus.SourceClassSynthetic},
		}, corpus.LoadOptions{})
	}
	matches, err := filepath.Glob(filepath.Join(path, "*.json"))
	if err != nil {
		return corpus.Suite{}, err
	}
	if len(matches) == 0 {
		return corpus.Suite{}, fmt.Errorf("no .json files in %s", path)
	}
	locs := make([]corpus.SourceLoc, 0, len(matches))
	for _, m := range matches {
		locs = append(locs, corpus.SourceLoc{Path: m, Class: corpus.SourceClassSynthetic})
	}
	return corpus.Load(locs, corpus.LoadOptions{})
}

// loadManifest reads the CONSENTED-case manifest. The manifest is
// itself a runnable fixture file; we re-use the corpus schema for
// integrity checking, but cases in the manifest are grouped under a
// CONSENTED provenance.
func loadManifest(path string) ([]corpus.Case, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m RealManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	out := []corpus.Case{}
	for _, e := range m.Cases {
		if e.CaseID == "" {
			return nil, errors.New("manifest: empty case_id")
		}
		if e.Provenance.Kind != corpus.ProvenanceConsented {
			return nil, fmt.Errorf("manifest case %q: provenance.kind must be CONSENTED", e.CaseID)
		}
		if e.Provenance.ConsentID == "" {
			return nil, fmt.Errorf("manifest case %q: missing consent_id", e.CaseID)
		}
		c := corpus.Case{
			ID:         e.CaseID,
			Split:      corpus.SplitEval,
			Provenance: e.Provenance,
			LangCohort: corpus.LangCohort{
				Language: e.Lang,
				Cohort:   corpus.CohortAdultSynthetic,
			},
			Category: corpus.Category(e.Category),
			Input: corpus.Input{
				Kind:          corpus.InputAudio,
				AudioDigest:   e.AudioDigest,
				AudioPath:     e.AudioPath,
				ContentType:   "audio/wav",
				AudioByteSize: 0,
			},
			Expected: e.Expected,
		}
		if err := c.Probe(); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func parseFilter(spec string) map[string]string {
	out := map[string]string{}
	if spec == "" {
		return out
	}
	parts := splitComma(spec)
	for _, p := range parts {
		k, v, ok := splitKV(p)
		if ok {
			out[k] = v
		}
	}
	return out
}

func splitComma(s string) []string {
	out := []string{}
	cur := ""
	for _, c := range s {
		if c == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func splitKV(s string) (string, string, bool) {
	eq := -1
	for i, c := range s {
		if c == '=' {
			eq = i
			break
		}
	}
	if eq < 0 {
		return "", "", false
	}
	return s[:eq], s[eq+1:], true
}
