package report

import (
	"strings"
	"testing"

	"sthira/backend/eval/corpus"
	"sthira/backend/eval/provider"
)

func mkCase(category corpus.Category, lang string, expected corpus.ExpectedOutcome) corpus.Case {
	return corpus.Case{
		ID:         "r." + string(category),
		LangCohort: corpus.LangCohort{Language: lang, Cohort: corpus.CohortAdultSynthetic},
		Category:   category,
		Split:      corpus.SplitEval,
		Provenance: corpus.Provenance{Kind: corpus.ProvenanceSynthetic, License: "SYNTHETIC"},
		Input:      corpus.Input{Kind: corpus.InputTranscript, Text: "x"},
		Expected:   corpus.Expected{Outcome: expected},
		Context:    corpus.Context{Language: lang, Places: []string{"PLACE-X"}},
	}
}

func TestReportPassAndFail(t *testing.T) {
	res := []provider.Result{}
	for i := 0; i < 5; i++ {
		res = append(res, provider.Result{
			Case:            mkCase(corpus.CategoryCodeSwitch, "ml-IN", corpus.ExpectOK),
			Outcome:         "OK",
			IntentObserved:  "FOCUS_PLACE",
			ActionsObserved: []string{"FOCUS_FEATURE:PLACE-X"},
			Reconciled:      true,
			ReconcileDetail: "OK",
			Stage:           provider.StageTimings{TotalMS: 100},
		})
	}
	// 5 OK-passes.
	r := Build(res)
	if r.Summary.Pass != 5 {
		t.Fatalf("pass=%d", r.Summary.Pass)
	}
	if r.Summary.Fail != 0 {
		t.Fatalf("fail=%d", r.Summary.Fail)
	}
	if r.Summary.FalseAcceptance == nil {
		t.Fatalf("FA rate must not be nil when n >= 5 (rate is 0/n)")
	}
	if r.Summary.FalseAcceptance.Num != 0 {
		t.Fatalf("FA num=%d expected 0", r.Summary.FalseAcceptance.Num)
	}
}

func TestReportFalseAcceptanceCounts(t *testing.T) {
	// 5 cases expected DATA_UNAVAILABLE; one of them answered OK.
	res := []provider.Result{}
	for i := 0; i < 4; i++ {
		res = append(res, provider.Result{
			Case:            mkCase(corpus.CategoryClaimedShortcut, "ml-IN", corpus.ExpectDATAUNAVAILABLE),
			Outcome:         "DATA_UNAVAILABLE",
			Reconciled:      true,
			ReconcileDetail: "DATA_UNAVAILABLE",
			Stage:           provider.StageTimings{TotalMS: 50},
		})
	}
	res = append(res, provider.Result{
		Case:            mkCase(corpus.CategoryClaimedShortcut, "ml-IN", corpus.ExpectDATAUNAVAILABLE),
		Outcome:         "OK",
		Reconciled:      false,
		ReconcileDetail: "expected DATA_UNAVAILABLE, got OK",
		Stage:           provider.StageTimings{TotalMS: 50},
	})
	r := Build(res)
	if r.Summary.FalseAcceptance == nil {
		t.Fatalf("FA required")
	}
	if r.Summary.FalseAcceptance.Num != 1 {
		t.Fatalf("FA num=%d", r.Summary.FalseAcceptance.Num)
	}
	if r.Summary.FalseAcceptance.Den != 5 {
		t.Fatalf("FA den=%d", r.Summary.FalseAcceptance.Den)
	}
}

func TestReportWarmUpExcluded(t *testing.T) {
	res := []provider.Result{
		{
			Case:     mkCase(corpus.CategoryCodeSwitch, "ml-IN", corpus.ExpectOK),
			Outcome:  "OK",
			WarmedUp: true,
			Stage:    provider.StageTimings{TotalMS: 80},
		},
		{
			Case:            mkCase(corpus.CategoryCodeSwitch, "ml-IN", corpus.ExpectOK),
			Outcome:         "OK",
			Reconciled:      true,
			ReconcileDetail: "OK",
			Stage:           provider.StageTimings{TotalMS: 100},
		},
	}
	r := Build(res)
	if r.Summary.WarmedUp != 1 {
		t.Fatalf("warmed_up=%d", r.Summary.WarmedUp)
	}
	if r.Summary.Total != 1 {
		t.Fatalf("total=%d", r.Summary.Total)
	}
}

func TestRateBelowFloorIsNil(t *testing.T) {
	if NewRate(1, 4) != nil {
		t.Fatalf("expected nil below floor")
	}
	if NewRate(1, 5) == nil {
		t.Fatalf("expected non-nil at floor")
	}
}

func TestReportWritesMarkdown(t *testing.T) {
	res := []provider.Result{
		{
			Case:            mkCase(corpus.CategoryCodeSwitch, "ml-IN", corpus.ExpectOK),
			Outcome:         "OK",
			IntentObserved:  "FOCUS_PLACE",
			Reconciled:      true,
			ReconcileDetail: "OK",
			Stage:           provider.StageTimings{TotalMS: 100},
		},
	}
	r := Build(res)
	var b strings.Builder
	r.WriteMarkdown(&b)
	s := b.String()
	if !strings.Contains(s, "P6 evaluation report") {
		t.Fatalf("missing title")
	}
	if !strings.Contains(s, "ml-IN") {
		t.Fatalf("missing language")
	}
}

func TestReportLatencyP95(t *testing.T) {
	res := []provider.Result{}
	for i := 0; i < 100; i++ {
		res = append(res, provider.Result{
			Case:            mkCase(corpus.CategoryCameraMove, "ml-IN", corpus.ExpectOK),
			Outcome:         "OK",
			IntentObserved:  "ZOOM",
			Reconciled:      true,
			ReconcileDetail: "OK",
			Stage:           provider.StageTimings{TotalMS: int64((i + 1) * 10)},
		})
	}
	r := Build(res)
	if r.Summary.LatencyP95MS < 900 {
		t.Fatalf("p95=%d want >= 900", r.Summary.LatencyP95MS)
	}
}
