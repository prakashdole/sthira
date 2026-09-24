// Command scenario-prep prepares scenario data for Sthira review. It runs
// entirely offline and never touches the database, model worker, or
// government endpoint.
//
// Usage:
//
//	scenario-prep init    --workspace DIR [--force]
//	scenario-prep validate --workspace DIR --index PATH
//	scenario-prep report  --workspace DIR --index PATH [--out-dir DIR]
//	scenario-prep bundle  --workspace DIR --index PATH --output DIR [--allow-draft]
//
// Exit codes (documented for automation):
//
//	0 SUCCESS    ready-to-review bundle produced
//	2 INVALID    structural validation failure
//	3 INCOMPLETE  structurally valid but missing data (bundle refused unless --allow-draft)
//	4 IO_ERROR   file system / permission / layout failure
//	1 OTHER      unexpected internal error or usage error
package main

import (
	"fmt"
	"io"
	"os"
)

// toolVersion is the published tool version; included in the bundle
// manifest and the human-readable report so downstream tooling can match.
const toolVersion = "0.1.0"

const usageText = `scenario-prep — offline data preparation CLI for Sthira exercise scenarios

Subcommands:

  init      produce a minimal DRAFT workspace the user fills in
  validate  structurally check a catalogue + index (no files written)
  report    write a machine-readable JSON + Markdown report
  bundle    publish a deterministic handoff directory for review

Run a subcommand with --help for its flags.

Exit codes:
  0  SUCCESS     ready bundle produced
  2  INVALID     structural validation failure
  3  INCOMPLETE  structurally valid but missing data
  4  IO_ERROR    file system / permission / layout failure
  1  OTHER       usage or internal error
`

func main() {
	if len(os.Args) < 2 {
		_, _ = io.WriteString(os.Stderr, usageText) // #nosec G104
		os.Exit(1)
	}
	switch os.Args[1] {
	case "-h", "--help", "help":
		_, _ = io.WriteString(os.Stdout, usageText) // #nosec G104
		os.Exit(0)
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "report":
		os.Exit(runReport(os.Args[2:]))
	case "bundle":
		os.Exit(runBundle(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "scenario-prep: unknown subcommand %q\n\n%s", os.Args[1], usageText)
		os.Exit(1)
	}
}

// printStderr writes a short banner to stderr so users see the failure
// mode even when stdout is captured.
func printStderr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "scenario-prep: "+format+"\n", args...)
}
