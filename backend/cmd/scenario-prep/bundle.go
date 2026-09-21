package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"sthira/backend/internal/scenarioprep"
)

const bundleHelp = `bundle — publish a deterministic handoff directory for review

Usage:
  scenario-prep bundle --workspace DIR --index PATH --output DIR [--allow-draft]

Flags:
  --workspace DIR  directory containing the catalogue and packages
  --index PATH     input index path
  --output DIR     destination directory; MUST NOT exist
  --allow-draft    publish a PREPARATION_DRAFT bundle when the report is INCOMPLETE

The handoff bundle is NOT the P5 signed offline-client package. It does
not claim P5 compatibility, never publishes to citizens, never activates
sources, never generates signing keys, and never adds signature
placeholders. Structurally invalid inputs cannot produce a ready bundle.
INCOMPLETE bundles are rejected unless --allow-draft is set.

Exit codes:
  0  PREPARATION_READY bundle produced
  2  INVALID inputs (bundle refused)
  3  INCOMPLETE inputs without --allow-draft (rejection summary written)
  4  IO_ERROR
`

func runBundle(args []string) int {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "workspace directory")
	idxPath := fs.String("index", "", "input index path")
	out := fs.String("output", "", "destination directory; refused if it already exists")
	allowDraft := fs.Bool("allow-draft", false, "publish a PREPARATION_DRAFT bundle when the report is INCOMPLETE")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, bundleHelp)
		return 1
	}
	if *ws == "" || *idxPath == "" || *out == "" {
		fmt.Fprint(os.Stderr, bundleHelp)
		printStderr("--workspace, --index and --output are required")
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
	absOut, err := filepath.Abs(*out)
	if err != nil {
		printStderr("output absolute path: %v", err)
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

	if err := scenarioprep.Bundle(absWS, absIdx, absOut, *allowDraft, rep, toolVersion); err != nil {
		var pe *scenarioprep.PrintableError
		if errors.As(err, &pe) {
			if errors.Is(pe.Unwrap(), scenarioprep.ErrIO) || errors.Is(pe.Unwrap(), scenarioprep.ErrUnsafeLayout) {
				printStderr("%v", err)
				return 4
			}
		}
		printStderr("bundle: %v", err)
		return 1
	}

	switch rep.Status {
	case scenarioprep.StatusReady:
		fmt.Printf("scenario-prep bundle: PREPARATION_READY -> %s\n", absOut)
		return 0
	case scenarioprep.StatusIncomplete:
		if *allowDraft {
			fmt.Printf("scenario-prep bundle: PREPARATION_DRAFT -> %s\n", absOut)
			return 3
		}
		fmt.Printf("scenario-prep bundle: rejected (INCOMPLETE) -> %s\n", absOut)
		return 3
	default:
		fmt.Printf("scenario-prep bundle: rejected (INVALID) -> %s\n", absOut)
		return 2
	}
}