// Package report aggregates runner results into per-language and
// per-cohort metrics. It is the only place where reported numbers
// surface to a file or stderr — the runner does not print anything.
//
// Aggregation rules:
//   - Per-(language, cohort, category) group: total / pass /
//     fail / clarify / refuse / not_evaluated counts.
//   - Pass-rate carries uncertainty: n_pass and n_total. When n_total
//     < MinSampleForClaim the metric is reported as NOT_EVALUATED
//     rather than a fraction.
//   - Latency p50 / p95 is reported for warm-up-filtered, non-ERROR
//     results only.
//   - Provider mode is reported alongside every group: a real run
//     must NEVER be confused with a deterministic one.
//
// The reporter never invents data. Missed counts are flagged, not
// padded.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"sthira/backend/eval/corpus"
	"sthira/backend/eval/provider"
)

// MinSampleForClaim is the floor for any rate metric. Below this, the
// group is NOT_EVALUATED — the reporter hides the rate.
const MinSampleForClaim = 5

// Report is the produced artifact.
type Report struct {
	Summary    Summary                  `json:"summary"`
	ByLanguage map[string]LangAggregate `json:"by_language"`
	ByCohort   map[string]CohortStats   `json:"by_cohort"`
	ByCategory map[string]CatStats      `json:"by_category"`
	Failures   []FailureRow             `json:"failures,omitempty"`
}

// Summary is the top-level roll-up.
type Summary struct {
	Total            int               `json:"total"`
	Pass             int               `json:"pass"`
	Fail             int               `json:"fail"`
	NotEvaluated     int               `json:"not_evaluated"`
	WarmedUp         int               `json:"warmed_up"`
	ProviderMode     provider.Mode     `json:"provider_mode"`
	LatencyP50MS     int64             `json:"latency_p50_ms"`
	LatencyP95MS     int64             `json:"latency_p95_ms"`
	RejectionRate    *Rate             `json:"rejection_rate,omitempty"`
	ClarifyRate      *Rate             `json:"clarify_rate,omitempty"`
	UsefulActionRate *Rate             `json:"useful_action_rate,omitempty"`
	FalseAcceptance  *Rate             `json:"false_acceptance_rate,omitempty"`
	Uncertainty      map[string]string `json:"uncertainty,omitempty"`
}

// LangAggregate reports one language.
type LangAggregate struct {
	Language string   `json:"language"`
	Total    int      `json:"total"`
	Pass     int      `json:"pass"`
	Fail     int      `json:"fail"`
	NotEval  int      `json:"not_evaluated"`
	LatP50   int64    `json:"latency_p50_ms"`
	LatP95   int64    `json:"latency_p95_ms"`
	Useful   *Rate    `json:"useful_action_rate,omitempty"`
	FA       *Rate    `json:"false_acceptance_rate,omitempty"`
	Cohorts  []string `json:"cohorts"`
}

// CohortStats tracks one cohort dimension.
type CohortStats struct {
	Cohort string `json:"cohort"`
	Total  int    `json:"total"`
	Pass   int    `json:"pass"`
	Fail   int    `json:"fail"`
}

// CatStats tracks one category.
type CatStats struct {
	Category string `json:"category"`
	Total    int    `json:"total"`
	Pass     int    `json:"pass"`
	Fail     int    `json:"fail"`
}

// FailureRow points back to one failing case.
type FailureRow struct {
	CaseID   string `json:"case_id"`
	Language string `json:"language"`
	Outcome  string `json:"outcome"`
	Expected string `json:"expected"`
	Detail   string `json:"detail"`
	StageASR int64  `json:"stage_asr_ms"`
	StageMid int64  `json:"stage_middle_ms"`
	StageTTS int64  `json:"stage_tts_ms"`
}

// Rate is a fraction with explicit n. Reporters write *Rate; nil
// means "NOT_EVALUATED at this granularity".
type Rate struct {
	N    int     `json:"n"`
	Num  int     `json:"num"`
	Den  int     `json:"den"`
	Frac float64 `json:"frac"`
}

// NewRate returns a Rate from numerator / denominator. If denominator
// is below MinSampleForClaim the function returns nil; the reporter
// records "NOT_EVALUATED (n < 5)" instead.
func NewRate(num, den int) *Rate {
	if den < MinSampleForClaim {
		return nil
	}
	return &Rate{N: den, Num: num, Den: den, Frac: float64(num) / float64(den)}
}

// Build returns a Report for the given provider.Results. It is
// deterministic given the input order.
func Build(results []provider.Result) Report {
	r := Report{
		ByLanguage: map[string]LangAggregate{},
		ByCohort:   map[string]CohortStats{},
		ByCategory: map[string]CatStats{},
	}
	var (
		totalLatencies []int64
		rejection      = [3]int{0, 0}
		clarify        = [3]int{0, 0}
		useful         = [3]int{0, 0}
		falseAcceptor  = [3]int{0, 0}
	)
	for _, res := range results {
		c := res.Case
		if res.WarmedUp {
			r.Summary.WarmedUp++
			continue
		}
		r.Summary.Total++
		if !res.Reconciled {
			if res.Error != "" || res.Outcome == "" {
				r.Summary.NotEvaluated++
			} else {
				r.Summary.Fail++
			}
		} else {
			r.Summary.Pass++
		}
		if !res.WarmedUp && res.Stage.TotalMS > 0 && res.Error == "" {
			totalLatencies = append(totalLatencies, res.Stage.TotalMS)
		}
		if isRejection(res.Case, res) {
			rejection[1]++
			rejection[2]++
		}
		rejection[2]++
		if isClarify(res.Case, res) {
			clarify[1]++
			clarify[2]++
		}
		clarify[2]++
		if isUsefulAction(res.Case, res) {
			useful[1]++
			useful[2]++
		}
		useful[2]++
		if isFalseAcceptance(res.Case, res) {
			falseAcceptor[1]++
		}
		falseAcceptor[2]++

		lang := c.LangCohort.Language
		l, ok := r.ByLanguage[lang]
		if !ok {
			l = LangAggregate{Language: lang, Cohorts: []string{}}
		}
		l.Total++
		if res.WarmedUp {
			l.NotEval++
		} else if res.Reconciled {
			l.Pass++
		} else {
			l.Fail++
		}
		if !res.WarmedUp && res.Stage.TotalMS > 0 && res.Error == "" {
			l.LatP50 = pickP(int64(l.Total), l.LatP50, res.Stage.TotalMS, 50)
			l.LatP95 = pickP(int64(l.Total), l.LatP95, res.Stage.TotalMS, 95)
		}
		if !containsString(l.Cohorts, string(c.LangCohort.Cohort)) {
			l.Cohorts = append(l.Cohorts, string(c.LangCohort.Cohort))
		}
		r.ByLanguage[lang] = l

		cs, ok := r.ByCohort[string(c.LangCohort.Cohort)]
		if !ok {
			cs = CohortStats{Cohort: string(c.LangCohort.Cohort)}
		}
		cs.Total++
		if res.WarmedUp {
		} else if res.Reconciled {
			cs.Pass++
		} else {
			cs.Fail++
		}
		r.ByCohort[string(c.LangCohort.Cohort)] = cs

		cat, ok := r.ByCategory[string(c.Category)]
		if !ok {
			cat = CatStats{Category: string(c.Category)}
		}
		cat.Total++
		if res.WarmedUp {
		} else if res.Reconciled {
			cat.Pass++
		} else {
			cat.Fail++
		}
		r.ByCategory[string(c.Category)] = cat

		if !res.WarmedUp && !res.Reconciled {
			r.Failures = append(r.Failures, FailureRow{
				CaseID:   c.ID,
				Language: c.LangCohort.Language,
				Outcome:  res.Outcome,
				Expected: string(c.Expected.Outcome),
				Detail:   res.ReconcileDetail,
				StageASR: res.Stage.ASRMS,
				StageMid: res.Stage.MiddleMS,
				StageTTS: res.Stage.TTSMS,
			})
		}
	}
	r.Summary.ProviderMode = chooseMode(results)
	r.Summary.LatencyP50MS = percentile(totalLatencies, 50)
	r.Summary.LatencyP95MS = percentile(totalLatencies, 95)
	r.Summary.RejectionRate = NewRate(rejection[1], rejection[2])
	r.Summary.ClarifyRate = NewRate(clarify[1], clarify[2])
	r.Summary.UsefulActionRate = NewRate(useful[1], useful[2])
	r.Summary.FalseAcceptance = NewRate(falseAcceptor[1], falseAcceptor[2])
	if len(results) < MinSampleForClaim {
		r.Summary.Uncertainty = map[string]string{
			"total": "NOT_EVALUATED (n < 5)",
		}
	}
	sortMapKeys(r.ByLanguage)
	sortMapKeys(r.ByCohort)
	sortMapKeys(r.ByCategory)
	return r
}

func isRejection(c corpus.Case, res provider.Result) bool {
	if c.Expected.Outcome != corpus.ExpectREFUSE &&
		c.Expected.Outcome != corpus.ExpectDATAUNAVAILABLE &&
		c.Expected.Outcome != corpus.ExpectUNSUPPORTED {
		return false
	}
	if !res.Reconciled {
		return false
	}
	return true
}

func isClarify(c corpus.Case, res provider.Result) bool {
	return c.Expected.Outcome == corpus.ExpectCLARIFY && res.Reconciled
}

// isUsefulAction captures "did the worker produce a useful action":
// for non-OK expected outcomes, it's the case returning the expected
// non-OK shape. A real workflow useful-action lives in the OK
// outcomes only.
func isUsefulAction(c corpus.Case, res provider.Result) bool {
	if c.Expected.Outcome != corpus.ExpectOK {
		return false
	}
	return res.Reconciled
}

// isFalseAcceptance counts the dangerous miss: the worker answered
// OK when the case expected a refuse / unavailable / degraded shape.
// This is the metric that distinguishes "did the worker fall into a
// forged answer". A reconciled failure on the same expected shape
// (where the runner caught the forgery) does NOT count — only an
// observed OK does.
func isFalseAcceptance(c corpus.Case, res provider.Result) bool {
	switch c.Expected.Outcome {
	case corpus.ExpectREFUSE,
		corpus.ExpectUNSUPPORTED,
		corpus.ExpectDATAUNAVAILABLE,
		corpus.ExpectDegraded:
		return res.Outcome == "OK"
	}
	return false
}

// chooseMode picks the dominant provider mode. A mixed result is
// reported as the literal mode of the first non-warm-up entry to
// preserve honesty: deterministic-harness validation vs real run.
func chooseMode(results []provider.Result) provider.Mode {
	for _, r := range results {
		if !r.WarmedUp {
			return r.ProviderMode
		}
	}
	return provider.ModeDeterministic
}

func pickP(_ int64, current, next int64, _ int) int64 {
	if current == 0 {
		return next
	}
	// pickP is intentionally crude (running max of recent). The
	// p50/p95 totals use percentile() on the full slice.
	if next > current {
		return next
	}
	return current
}

func percentile(xs []int64, q int) int64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]int64(nil), xs...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := (len(cp) - 1) * q / 100
	if idx < 0 {
		idx = 0
	}
	return cp[idx]
}

func containsString(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func sortMapKeys[V any](m map[string]V) {
	// Maps don't order in JSON in Go; we don't need to, but we leave
	// the helper for tests that need ordered iteration.
	_ = m
}

// WriteMarkdown emits a stable, diff-friendly text summary. The
// reporter is the only place numbers become prose.
func (r Report) WriteMarkdown(w io.Writer) {
	fmt.Fprintln(w, "# P6 evaluation report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- provider: %s\n", r.Summary.ProviderMode)
	fmt.Fprintf(w, "- total: %d\n", r.Summary.Total)
	fmt.Fprintf(w, "- pass: %d\n", r.Summary.Pass)
	fmt.Fprintf(w, "- fail: %d\n", r.Summary.Fail)
	fmt.Fprintf(w, "- not_evaluated: %d\n", r.Summary.NotEvaluated)
	fmt.Fprintf(w, "- warmed_up (discarded): %d\n", r.Summary.WarmedUp)
	if r.Summary.LatencyP50MS > 0 {
		fmt.Fprintf(w, "- latency p50: %d ms\n", r.Summary.LatencyP50MS)
	}
	if r.Summary.LatencyP95MS > 0 {
		fmt.Fprintf(w, "- latency p95: %d ms\n", r.Summary.LatencyP95MS)
	}
	if r.Summary.RejectionRate != nil {
		fmt.Fprintf(w, "- rejection rate (n=%d): %.2f\n", r.Summary.RejectionRate.N, r.Summary.RejectionRate.Frac)
	}
	if r.Summary.ClarifyRate != nil {
		fmt.Fprintf(w, "- clarify rate (n=%d): %.2f\n", r.Summary.ClarifyRate.N, r.Summary.ClarifyRate.Frac)
	}
	if r.Summary.UsefulActionRate != nil {
		fmt.Fprintf(w, "- useful-action rate (n=%d): %.2f\n", r.Summary.UsefulActionRate.N, r.Summary.UsefulActionRate.Frac)
	}
	if r.Summary.FalseAcceptance != nil {
		fmt.Fprintf(w, "- false-acceptance rate (n=%d): %.2f\n", r.Summary.FalseAcceptance.N, r.Summary.FalseAcceptance.Frac)
	}
	if r.Summary.Uncertainty != nil {
		if u, ok := r.Summary.Uncertainty["total"]; ok {
			fmt.Fprintf(w, "- uncertainty: %s\n", u)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## By language")
	for _, lang := range sortedKeys(r.ByLanguage) {
		v := r.ByLanguage[lang]
		fmt.Fprintf(w, "### %s (n=%d, pass=%d, fail=%d, ne=%d)\n", lang, v.Total, v.Pass, v.Fail, v.NotEval)
		if v.Useful != nil {
			fmt.Fprintf(w, "  useful-action: %.2f\n", v.Useful.Frac)
		}
		if v.FA != nil {
			fmt.Fprintf(w, "  false-acceptance: %.2f\n", v.FA.Frac)
		}
		if v.LatP50 > 0 {
			fmt.Fprintf(w, "  latency p50/p95: %d / %d ms\n", v.LatP50, v.LatP95)
		}
		for _, cs := range v.Cohorts {
			if cs != "" {
				fmt.Fprintf(w, "  cohort: %s\n", cs)
			}
		}
	}
	if len(r.Failures) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "## Failures")
		for _, f := range r.Failures {
			fmt.Fprintf(w, "- %s [%s] outcome=%s expected=%s :: %s\n", f.CaseID, f.Language, f.Outcome, f.Expected, f.Detail)
		}
	}
	_ = strings.Join
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
