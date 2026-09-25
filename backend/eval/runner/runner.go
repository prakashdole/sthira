// Package runner drives the P6 evaluation harness. It loads a corpus,
// applies per-run filters, dispatches each case through a Provider,
// captures per-stage timings, reconciles against the case's locked
// Expected outcome, and emits a Result for the reporter.
//
// Two modes:
//
//   - ModeSynthetic: SYNTHETIC fixtures only, no real workers needed.
//     Default is the Deterministic provider.
//   - ModeReal:       real worker run, behind the -real-inference flag.
//     Synthetic fixtures are filtered out; missing reviewers/samples
//     record NOT_EVALUATED, never zero-failures or 100% success.
//
// Warm/Cold: a WarmRuns count dispatches before timing measurement.
// The runner reports ColdRun=true on the first per-(language,category)
// dispatch.
package runner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"sthira/backend/eval/corpus"
	"sthira/backend/eval/provider"
)

// Mode separates runner modes.
type Mode string

const (
	ModeSynthetic Mode = "SYNTHETIC"
	ModeReal      Mode = "REAL"
)

// WarmGate classifies a dispatch's readiness for latency reporting.
type WarmGate string

const (
	WarmCold   WarmGate = "COLD"
	WarmWarm   WarmGate = "WARM"
	WarmForced WarmGate = "FORCED"
)

// StageRun selects which pipeline stages to invoke.
type StageRun struct {
	ASR    bool
	Middle bool
	TTS    bool
}

// AuditEvent is one entry the runner logs.
type AuditEvent struct {
	Stage   string
	Case    string
	Status  string
	Elapsed time.Duration
	Note    string
}

// Config wires a runner.
type Config struct {
	Mode      Mode
	Provider  provider.Provider
	Filter    map[string]string
	Split     string // "" = no filter; "EVAL" / "DEV" / "TRAIN"
	WarmRuns  int
	BudgetMS  int
	StagesRun StageRun
	Now       func() time.Time
}

// DefaultConfig returns the synthetic / deterministic / all-stages config.
func DefaultConfig() Config {
	return Config{
		Mode:      ModeSynthetic,
		Provider:  provider.NewDeterministic(),
		WarmRuns:  1,
		BudgetMS:  1500,
		StagesRun: StageRun{ASR: true, Middle: true, TTS: true},
		Now:       func() time.Time { return time.Now().UTC() },
	}
}

// Runner drives a Config.
type Runner struct {
	cfg Config
}

// New returns a Runner.
func New(cfg Config) *Runner {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.Provider == nil {
		cfg.Provider = provider.NewDeterministic()
	}
	if cfg.WarmRuns < 0 {
		cfg.WarmRuns = 0
	}
	if cfg.BudgetMS <= 0 {
		cfg.BudgetMS = 1500
	}
	if !cfg.StagesRun.ASR && !cfg.StagesRun.Middle && !cfg.StagesRun.TTS {
		cfg.StagesRun = StageRun{ASR: true, Middle: true, TTS: true}
	}
	return &Runner{cfg: cfg}
}

// Run walks every case, dispatches, reconciles.
// Returns Results in input order.
func (r *Runner) Run(ctx context.Context, suite corpus.Suite) ([]provider.Result, error) {
	if r.cfg.Provider == nil {
		return nil, errors.New("runner: provider nil")
	}
	results := make([]provider.Result, 0, len(suite.Cases))
	seen := map[string]struct{}{}
	for _, c := range suite.Cases {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		if !r.eligible(c) {
			continue
		}
		warmKey := c.LangCohort.Language + "|" + string(c.Category)
		if _, ok := seen[warmKey]; !ok {
			for i := 0; i < r.cfg.WarmRuns; i++ {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				warm := r.dispatch(ctx, c, WarmForced)
				results = append(results, warm)
			}
			seen[warmKey] = struct{}{}
		}
		gate := WarmWarm
		if r.cfg.WarmRuns == 0 {
			gate = WarmCold
		}
		res := r.dispatch(ctx, c, gate)
		results = append(results, res)
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Case.ID < results[j].Case.ID })
	return results, nil
}

// eligible filters by Mode / Split / corpus.IsRelevant.
func (r *Runner) eligible(c corpus.Case) bool {
	if r.cfg.Mode == ModeReal {
		if c.Provenance.Kind != corpus.ProvenanceConsented {
			return false
		}
	} else {
		if c.Provenance.Kind != corpus.ProvenanceSynthetic {
			return false
		}
	}
	if r.cfg.Split != "" && string(c.Split) != r.cfg.Split {
		return false
	}
	if !c.IsRelevant(r.cfg.Filter) {
		return false
	}
	return true
}

// dispatch runs the case through the configured stages and returns
// the fully-populated Result.
func (r *Runner) dispatch(ctx context.Context, c corpus.Case, gate WarmGate) provider.Result {
	res := provider.Result{
		Case:         c,
		ProviderMode: r.cfg.Provider.Mode(),
		ColdRun:      gate == WarmCold,
		WarmedUp:     gate == WarmForced,
	}
	stage := provider.NewTimings()
	timeout := time.Duration(safeBudget(c, r.cfg.BudgetMS)) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(r.cfg.BudgetMS) * time.Millisecond
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var asr provider.ASROutcome
	var mid provider.MiddleOutcome

	if r.cfg.StagesRun.ASR {
		t := r.cfg.Now()
		out, err := r.cfg.Provider.ASR(cctx, provider.ASRRequest{
			RequestID:   c.ID,
			Language:    c.LangCohort.Language,
			ContentType: c.Input.ContentType,
			Case:        c,
		})
		stage.ASRMS = time.Since(t).Milliseconds()
		if err != nil {
			res.Error = "asr:" + err.Error()
			res.Outcome = "ERROR"
			res.Stage = stage
			res.Reconciled, res.ReconcileDetail = reconcile(c, res)
			return res
		}
		asr = out
	}

	if r.cfg.StagesRun.Middle {
		t := r.cfg.Now()
		out, err := r.cfg.Provider.Middle(cctx, provider.MiddleRequest{
			RequestID:  c.ID,
			Language:   c.LangCohort.Language,
			Transcript: asr.Text,
			Context:    r.contextForCase(c),
			Case:       c,
		})
		stage.MiddleMS = time.Since(t).Milliseconds()
		if err != nil {
			res.Error = "middle:" + err.Error()
			res.Stage = stage
			res.Reconciled, res.ReconcileDetail = reconcile(c, res)
			return res
		}
		mid = out
		res.Outcome = mid.Status
		res.IntentObserved = mid.Intent
		res.ActionsObserved = mid.Actions
		res.SpeechObserved = mid.SpeechKey
		res.ClarifyObserved = mid.ClarifyIDs
		res.EvidenceObserved = mid.EvidenceIDs
		// Push ASR stage-2 reconciliation if we caught a UNSUPPORTED.
		if asr.State == "UNSUPPORTED_LANGUAGE" && mid.Status == "" {
			res.Outcome = "UNSUPPORTED_LANGUAGE"
		}
	}

	if r.cfg.StagesRun.TTS && !skipTTS(c, mid.Status) {
		t := r.cfg.Now()
		_, err := r.cfg.Provider.TTS(cctx, provider.TTSRequest{
			RequestID:       c.ID,
			SpeechKey:       mid.SpeechKey,
			Language:        c.LangCohort.Language,
			Text:            c.Context.TemplateText,
			Voice:           c.Context.Voice,
			SampleRate:      c.Context.SampleRate,
			Args:            map[string]any{},
			SourceVersion:   c.Context.SourceVersion,
			TemplateVersion: c.Context.TemplateVersion,
			Case:            c,
		})
		stage.TTSMS = time.Since(t).Milliseconds()
		if err != nil {
			res.Error = "tts:" + err.Error()
			res.Stage = stage
			res.Reconciled, res.ReconcileDetail = reconcile(c, res)
			return res
		}
		_ = c
	}

	stage.TotalMS = stage.ASRMS + stage.MiddleMS + stage.TTSMS
	res.Stage = stage
	res.Reconciled, res.ReconcileDetail = reconcile(c, res)
	return res
}

// contextForCase converts the case's context to a shape the worker
// accepts (place/red/.../facility ID slices).
func (r *Runner) contextForCase(c corpus.Case) map[string]any {
	return map[string]any{
		"language":       c.LangCohort.Language,
		"template_keys":  c.Context.TemplateKeys,
		"places":         c.Context.Places,
		"red_zones":      c.Context.RedZones,
		"safe_zones":     c.Context.SafeZones,
		"routes":         c.Context.Routes,
		"facilities":     c.Context.Facilities,
		"data_version":   c.Context.DataVersion,
		"source_version": c.Context.SourceVersion,
	}
}

// skipTTS decides whether the runner should invoke the TTS stage.
func skipTTS(c corpus.Case, observedStatus string) bool {
	if c.Category == corpus.CategoryCameraMove {
		return true
	}
	switch observedStatus {
	case "CANCELED", "ERROR", "":
		return true
	}
	if c.Expected.Outcome == corpus.ExpectCancel ||
		c.Expected.Outcome == corpus.ExpectERROR {
		return true
	}
	return false
}

// reconcile compares the observation against the case's locked
// Expected outcome. NOT_EVALUATED lives outside this function — the
// reporter accounts for it when the runner produced no outcome (a
// pre-stage error, ErrUnsupported, etc.).
func reconcile(c corpus.Case, res provider.Result) (bool, string) {
	switch c.Expected.Outcome {
	case corpus.ExpectOK:
		if res.Outcome != "OK" {
			return false, fmt.Sprintf("status=%s expected=OK", res.Outcome)
		}
		if len(c.Expected.Intents) > 0 {
			matched := false
			for _, want := range c.Expected.Intents {
				if intentMatched(res, want) {
					matched = true
					break
				}
			}
			if !matched {
				return false, "no expected intent matched (intent=" + res.IntentObserved + ", actions=" + join(res.ActionsObserved, ",") + ")"
			}
		}
		if len(c.Expected.SpeechKeys) > 0 {
			matched := false
			for _, want := range c.Expected.SpeechKeys {
				if res.SpeechObserved == want.Key {
					matched = true
					break
				}
			}
			if !matched {
				return false, "speech_key=" + res.SpeechObserved + " did not match expected"
			}
		}
		for _, ban := range c.Expected.RejectIDs {
			for _, action := range res.ActionsObserved {
				if containsID(action, ban) {
					return false, "rejected ID " + ban + " leaked into " + action
				}
			}
		}
		return true, "OK+actions+speech"
	case corpus.ExpectCLARIFY:
		if res.Outcome != "CLARIFY" {
			return false, fmt.Sprintf("status=%s expected=CLARIFY", res.Outcome)
		}
		for _, want := range c.Expected.ClarifyIDs {
			found := false
			for _, got := range res.ClarifyObserved {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				return false, "missing expected clarify_id " + want
			}
		}
		for _, ban := range c.Expected.RejectIDs {
			for _, action := range res.ActionsObserved {
				if containsID(action, ban) {
					return false, "rejected ID " + ban + " leaked into " + action
				}
			}
		}
		return true, "CLARIFY+ids"
	case corpus.ExpectUNSUPPORTED:
		if res.Outcome == "UNSUPPORTED" || res.Outcome == "UNSUPPORTED_LANGUAGE" {
			return true, "UNSUPPORTED"
		}
		return false, fmt.Sprintf("status=%s expected=UNSUPPORTED", res.Outcome)
	case corpus.ExpectDATAUNAVAILABLE:
		if res.Outcome == "DATA_UNAVAILABLE" {
			return true, "DATA_UNAVAILABLE"
		}
		return false, fmt.Sprintf("status=%s expected=DATA_UNAVAILABLE", res.Outcome)
	case corpus.ExpectERROR:
		return res.Outcome == "ERROR" || res.Error != "", "expected ERROR"
	case corpus.ExpectREFUSE:
		if res.Outcome == "UNSUPPORTED" || res.Outcome == "DATA_UNAVAILABLE" {
			return true, "REFUSE honored"
		}
		return false, fmt.Sprintf("status=%s did not refuse", res.Outcome)
	case corpus.ExpectCancel:
		if res.Outcome == "CANCELED" {
			return true, "CANCELED"
		}
		return false, fmt.Sprintf("status=%s expected=CANCELED", res.Outcome)
	case corpus.ExpectDegraded:
		if res.Outcome == "DEGRADED" {
			return true, "DEGRADED"
		}
		return false, fmt.Sprintf("status=%s expected=DEGRADED", res.Outcome)
	}
	return false, "no reconcile rule for outcome " + string(c.Expected.Outcome)
}

// intentMatched reports whether the observation's intent or any
// action shape corresponds to `want`. The lookup is by exact intent
// match, then by action-type prefix (e.g. `FOCUS_FEATURE` is the
// typed shape for intent `FOCUS_PLACE`) or action-type equality.
func intentMatched(res provider.Result, want string) bool {
	if res.IntentObserved == want {
		return true
	}
	for _, a := range res.ActionsObserved {
		if a == want {
			return true
		}
		if strings.HasPrefix(a, want+":") {
			return true
		}
		if strings.HasPrefix(a, want+"_") || strings.HasSuffix(a, "_"+want) {
			return true
		}
	}
	return false
}

// containsID returns true if `id` appears as a typed-target inside
// the action string. The action encoding is `TYPE:TARGET` or
// `TYPE:TARGET:EXTRA`.
func containsID(action, id string) bool {
	if action == "" || id == "" {
		return false
	}
	if action == id {
		return true
	}
	return strings.HasSuffix(action, ":"+id) ||
		strings.Contains(action, ":"+id+":")
}

func safeBudget(c corpus.Case, def int) int {
	if c.Expected.LatencyBudgetMS > 0 {
		return c.Expected.LatencyBudgetMS
	}
	return def
}

func join(xs []string, sep string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += sep
		}
		out += x
	}
	return out
}

// silence used import — runner keeps `sync` reserved for future
// concurrency work (e.g. parallel case dispatch with bounded slot).
var _ = sync.Mutex{}
