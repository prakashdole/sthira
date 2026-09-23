package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sthira/backend/internal/scenarioprep"
)

const reportHelp = `report — write a JSON + Markdown data-preparation report

Usage:
  scenario-prep report --workspace DIR --index PATH [--out-dir DIR]

Flags:
  --workspace DIR  directory containing the catalogue and packages
  --index PATH     input index path
  --out-dir DIR    output directory for report files (default: <workspace>/reports)

Files produced:
  report.json   machine-readable, same shape as validate output
  report.md     human-readable summary

Exit codes:
  0  ready
  2  INVALID
  3  INCOMPLETE
  4  IO_ERROR
`

func runReport(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "workspace directory")
	idxPath := fs.String("index", "", "input index path")
	outDir := fs.String("out-dir", "", "output directory (default: <workspace>/reports)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, reportHelp)
		return 1
	}
	if *ws == "" || *idxPath == "" {
		fmt.Fprint(os.Stderr, reportHelp)
		printStderr("--workspace and --index are required")
		return 1
	}
	absWS, err := absWorkspace(*ws)
	if err != nil {
		printStderr("%v", err)
		return 4
	}
	absIdx, err := resolveIndexPath(*idxPath, absWS)
	if err != nil {
		printStderr("%v", err)
		return 4
	}
	dst := *outDir
	if dst == "" {
		dst = filepath.Join(absWS, "reports")
	} else if !filepath.IsAbs(dst) {
		dst, err = filepath.Abs(dst)
		if err != nil {
			printStderr("output absolute path: %v", err)
			return 4
		}
	}
	if fileExistsCheck(dst) {
		printStderr("refusing to overwrite existing output directory %s", dst)
		return 4
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		printStderr("mkdir %s: %v", dst, err)
		return 4
	}

	rep, err := scenarioprep.Prepare(absWS, absIdx)
	if err != nil {
		var pe *scenarioprep.PrintableError
		if errors.As(err, &pe) {
			if errors.Is(pe.Unwrap(), scenarioprep.ErrIO) {
				printStderr("%v", err)
				return 4
			}
			printStderr("%v", err)
			return 2
		}
		printStderr("prepare: %v", err)
		return 1
	}

	jsonBytes, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		printStderr("encode json: %v", err)
		return 1
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := writeFileAtomic(filepath.Join(dst, "report.json"), jsonBytes); err != nil {
		printStderr("write report.json: %v", err)
		return 4
	}
	md := renderMarkdownReport(rep)
	if err := writeFileAtomic(filepath.Join(dst, "report.md"), []byte(md)); err != nil {
		printStderr("write report.md: %v", err)
		return 4
	}

	fmt.Printf("scenario-prep report: status=%s -> %s\n", rep.Status, dst)
	return scenarioprep.ExitCode(rep.Status)
}

func renderMarkdownReport(rep *scenarioprep.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Scenario preparation report\n\n")
	fmt.Fprintf(&b, "- status: **%s**\n", rep.Status)
	fmt.Fprintf(&b, "- catalogue: %s\n", rep.CataloguePath)
	fmt.Fprintf(&b, "- index: %s\n", rep.IndexPath)
	fmt.Fprintf(&b, "- exercise_clock: %s\n", emptyDash(rep.ExerciseClock))
	fmt.Fprintf(&b, "- state_count: %d\n", rep.StateCount)
	fmt.Fprintf(&b, "- scenario_count: %d\n", rep.ScenarioCount)
	fmt.Fprintf(&b, "- historical_count: %d\n", rep.HistoricalCount)
	fmt.Fprintf(&b, "- signature_status: %s\n\n", rep.SignatureStatus)

	if len(rep.States) > 0 {
		fmt.Fprintf(&b, "## States\n\n")
		for _, st := range rep.States {
			fmt.Fprintf(&b, "### %s (%s)\n\n", st.StateCode, emptyDash(st.Name))
			fmt.Fprintf(&b, "- scenarios: %d\n", len(st.Scenarios))
			if len(st.LanguagesClaimed) > 0 {
				fmt.Fprintf(&b, "- languages claimed: %s\n", strings.Join(st.LanguagesClaimed, ", "))
			}
			if len(st.LanguagesEvidenced) > 0 {
				fmt.Fprintf(&b, "- languages evidenced: %s\n", strings.Join(st.LanguagesEvidenced, ", "))
			}
			if len(st.LanguagesUnknown) > 0 {
				fmt.Fprintf(&b, "- languages unknown: %s\n", strings.Join(st.LanguagesUnknown, ", "))
			}
			fmt.Fprintf(&b, "\n")
		}
	}

	if len(rep.MissingReferences) > 0 {
		fmt.Fprintf(&b, "## Missing package references\n\n")
		for _, m := range dedupSorted(rep.MissingReferences) {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(rep.UnresolvedGates) > 0 {
		fmt.Fprintf(&b, "## Unresolved external gates\n\n")
		for _, g := range rep.UnresolvedGates {
			fmt.Fprintf(&b, "- %s\n", g)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(rep.Findings) > 0 {
		errs, warns, infos := groupBySeverity(rep.Findings)
		fmt.Fprintf(&b, "## Findings (%d)\n\n", len(rep.Findings))
		writeFindingList(&b, "Errors", errs)
		writeFindingList(&b, "Warnings", warns)
		writeFindingList(&b, "Info", infos)
	} else {
		fmt.Fprintf(&b, "## Findings\n\nNone.\n")
	}
	fmt.Fprintf(&b, "\n_This is a data-preparation report, not a completion certificate._\n")
	return b.String()
}

func writeFindingList(b *strings.Builder, label string, fs []scenarioprep.Finding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s (%d)\n\n", label, len(fs))
	for _, f := range fs {
		scope := ""
		if f.Scope != "" {
			scope = f.Scope + ": "
		}
		fmt.Fprintf(b, "- [%s] %s%s\n", f.Code, scope, f.Detail)
	}
	fmt.Fprintf(b, "\n")
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// writeFileAtomic writes data to a temp file in the same directory and
// renames into place. Avoids partial reads if a concurrent consumer is
// watching the destination.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".scenario-prep-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		cleanup()
		return err
	}
	return os.Rename(tmpName, path)
}

func fileExistsCheck(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}
