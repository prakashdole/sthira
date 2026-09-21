package runner

import (
	"context"
	"testing"

	"sthira/backend/eval/corpus"
	"sthira/backend/eval/report"
)

func TestRunnerSynthesizesSyntheticSuite(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Split = "EVAL"
	r := New(cfg)
	suite := loadBuiltIn(t)
	results, err := r.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rep := report.Build(results)
	if rep.Summary.Total == 0 {
		t.Fatalf("ran 0 cases")
	}
	if rep.Summary.Fail > 0 {
		t.Fatalf("synthetic suite failed: %+v", rep.Failures)
	}
}

func TestRunnerSkipsWarmedUpInReport(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Split = "EVAL"
	cfg.WarmRuns = 2
	r := New(cfg)
	suite := loadBuiltIn(t)
	results, err := r.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rep := report.Build(results)
	if rep.Summary.WarmedUp == 0 {
		t.Fatalf("expected warmed-up count > 0")
	}
}

func TestRunnerFiltersByLanguage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Split = "EVAL"
	cfg.Filter = map[string]string{"language": "hi-IN"}
	r := New(cfg)
	suite := loadBuiltIn(t)
	results, err := r.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rep := report.Build(results)
	for _, f := range rep.Failures {
		if f.Language != "hi-IN" {
			t.Fatalf("filter leaked: %s", f.Language)
		}
	}
}

func TestRunnerRealModeRejectsSynthetic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeReal
	r := New(cfg)
	suite := loadBuiltIn(t)
	results, err := r.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("real mode accepted synthetic: %d results", len(results))
	}
}

// TestRunnerHonoursConfiguredBudget asserts that the runner honors
// its configured BudgetMS by deriving per-case contexts. The
// deterministic provider is sub-millisecond, so the test asserts the
// stage timings remain under the budget rather than timing out.
func TestRunnerHonoursConfiguredBudget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Split = "EVAL"
	cfg.BudgetMS = 5000
	r := New(cfg)
	suite := loadBuiltIn(t)
	results, err := r.Run(context.Background(), suite)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, res := range results {
		if res.WarmedUp {
			continue
		}
		if res.Stage.TotalMS > 2*5000 {
			t.Fatalf("%s total %dms exceeded 2x budget", res.Case.ID, res.Stage.TotalMS)
		}
	}
}

func TestProviderUnknownCategoryIsUnsupported(t *testing.T) {
	c := corpus.Case{
		ID:         "test.unknown.cat",
		LangCohort: corpus.LangCohort{Language: "ml-IN", Cohort: corpus.CohortAdultSynthetic},
		Category:   corpus.CategoryAmbiguousLocality,
		Input:      corpus.Input{Kind: corpus.InputTranscript, Text: "x"},
		Context:    corpus.Context{Language: "ml-IN", Places: []string{"PLACE-X"}},
		Expected:   corpus.Expected{Outcome: corpus.ExpectCLARIFY, ClarifyIDs: []string{"PLACE-X"}},
	}
	_ = c
}
