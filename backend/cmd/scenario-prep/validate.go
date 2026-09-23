package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"sthira/backend/internal/scenarioprep"
)

const validateHelp = `validate — structural and cross-reference check; no files written

Usage:
  scenario-prep validate --workspace DIR --index PATH

Flags:
  --workspace DIR  directory containing the catalogue and packages
  --index PATH     workspace-relative or absolute path to the input index

Exit codes:
  0  all inputs validate, scenarios map cleanly to packages
  2  INVALID — structural validation failure
  3  INCOMPLETE — structurally valid but missing data (run report for detail)
  4  IO_ERROR
`

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "workspace directory")
	idxPath := fs.String("index", "", "input index path")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, validateHelp)
		return 1
	}
	if *ws == "" || *idxPath == "" {
		fmt.Fprint(os.Stderr, validateHelp)
		printStderr("--workspace and --index are required")
		return 1
	}
	absWS, err := filepath.Abs(*ws)
	if err != nil {
		printStderr("workspace absolute path: %v", err)
		return 4
	}
	absIdx, err := resolveIndexPath(*idxPath, absWS)
	if err != nil {
		printStderr("%v", err)
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
			// Malformed inputs (duplicate keys, unknown fields, missing
			// fields) reach here as a printable preparation error; the
			// structural failure means INVALID.
			printStderr("%v", err)
			return 2
		}
		printStderr("prepare: %v", err)
		return 1
	}

	fmt.Printf("scenario-prep validate: status=%s states=%d scenarios=%d historical=%d findings=%d\n",
		rep.Status, rep.StateCount, rep.ScenarioCount, rep.HistoricalCount, len(rep.Findings))
	errs, warns, infos := groupBySeverity(rep.Findings)
	if len(errs) > 0 {
		fmt.Println("errors:")
		for _, f := range errs {
			fmt.Printf("  [%s] %s%s\n", f.Code, scopePrefix(f.Scope), f.Detail)
		}
	}
	if len(warns) > 0 {
		fmt.Println("warnings:")
		for _, f := range warns {
			fmt.Printf("  [%s] %s%s\n", f.Code, scopePrefix(f.Scope), f.Detail)
		}
	}
	if len(infos) > 0 {
		fmt.Println("info:")
		for _, f := range infos {
			fmt.Printf("  [%s] %s%s\n", f.Code, scopePrefix(f.Scope), f.Detail)
		}
	}
	return scenarioprep.ExitCode(rep.Status)
}

// resolveIndexPath returns the absolute path of the index. When the input
// is relative, it is interpreted relative to the workspace (not the
// current shell directory) so the tool is reproducible.
func resolveIndexPath(p, ws string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("--index is required")
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(filepath.Join(ws, p))
}

func absWorkspace(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("--workspace is required")
	}
	return filepath.Abs(p)
}

func groupBySeverity(fs []scenarioprep.Finding) (errs, warns, infos []scenarioprep.Finding) {
	for _, f := range fs {
		switch f.Severity {
		case scenarioprep.SeverityError:
			errs = append(errs, f)
		case scenarioprep.SeverityWarning:
			warns = append(warns, f)
		case scenarioprep.SeverityInfo:
			infos = append(infos, f)
		}
	}
	return
}

func scopePrefix(s string) string {
	if s == "" {
		return ""
	}
	return s + ": "
}

// dedupSorted returns a sorted, deduplicated copy of in.
func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	m := map[string]bool{}
	for _, s := range in {
		m[s] = true
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
