package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"sthira/backend/internal/opkg"
)

// init produces a minimal DRAFT workspace. The user replaces DRAFT
// placeholders with real data before validation will accept the
// catalogue as launch-ready.
//
// The DRAFT markers and the help text are deliberately small: the tool
// does not choose the user's 10–15 states, invent routes/zones, or claim
// model language support.
const initHelp = `init — produce a minimal DRAFT workspace

Usage:
  scenario-prep init --workspace DIR [--force]

Flags:
  --workspace DIR  destination directory; created if missing
  --force          overwrite existing draft files

The workspace contains:
  README.md                user-facing instructions (what to supply)
  catalogue.template.json  DRAFT catalogue skeleton (invalid until filled)
  package.template.json    DRAFT operational-package skeleton
  index.template.json      DRAFT input index mapping scenarios -> packages
  examples/                one small clearly synthetic example

DRAFT markers are required: a placeholder catalogue without them
will not validate as a launch-ready artefact. This tool never
chooses the user's selected states or claims language support.
`

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	ws := fs.String("workspace", "", "destination workspace directory")
	force := fs.Bool("force", false, "overwrite existing draft files in the workspace")
	if err := fs.Parse(args); err != nil {
		fmt.Fprint(os.Stderr, initHelp)
		return 1
	}
	if *ws == "" {
		fmt.Fprint(os.Stderr, initHelp)
		printStderr("--workspace is required")
		return 1
	}
	if !filepath.IsAbs(*ws) {
		abs, err := filepath.Abs(*ws)
		if err != nil {
			printStderr("absolute path: %v", err)
			return 4
		}
		*ws = abs
	}

	if err := os.MkdirAll(*ws, 0o755); err != nil {
		printStderr("mkdir %s: %v", *ws, err)
		return 4
	}

	files := []struct {
		name string
		body []byte
	}{
		{"README.md", []byte(readmeBody)},
		{"catalogue.template.json", []byte(catalogueTemplate)},
		{"package.template.json", []byte(packageTemplate)},
		{"index.template.json", []byte(indexTemplate)},
	}
	for _, f := range files {
		dst := filepath.Join(*ws, f.name)
		if _, err := os.Stat(dst); err == nil && !*force {
			printStderr("refusing to overwrite existing %s (use --force to override)", dst)
			return 4
		}
		if err := os.WriteFile(dst, f.body, 0o644); err != nil {
			printStderr("write %s: %v", dst, err)
			return 4
		}
	}
	examplesDir := filepath.Join(*ws, "examples")
	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		printStderr("mkdir examples: %v", err)
		return 4
	}

	type exampleFile struct {
		name string
		body []byte
	}
	exs := []exampleFile{
		{"catalogue.json", []byte(exampleCatalogue)},
		{"index.json", []byte(exampleIndex)},
	}
	for _, e := range exs {
		dst := filepath.Join(examplesDir, e.name)
		if _, err := os.Stat(dst); err == nil && !*force {
			continue
		}
		if err := os.WriteFile(dst, e.body, 0o644); err != nil {
			printStderr("write %s: %v", dst, err)
			return 4
		}
	}

	// Build the two example packages programmatically so the embedded
	// checksum stays correct without hand-computed magic.
	if err := writeExamplePackage(filepath.Join(examplesDir, "pkg-sc-synth-1.json"), exampleSyntheticPackage("PKG-EX-SYNTH-1", "EX"), *force); err != nil {
		printStderr("write synth package: %v", err)
		return 4
	}
	if err := writeExamplePackage(filepath.Join(examplesDir, "pkg-sc-hist-1.json"), exampleHistoricalPackage("PKG-EX-HIST-1", "EX"), *force); err != nil {
		printStderr("write hist package: %v", err)
		return 4
	}

	fmt.Printf("scenario-prep: DRAFT workspace created at %s\n", *ws)
	fmt.Println("scenario-prep: fill in the templates with real user-supplied data; see README.md")
	return 0
}

// writeExamplePackage writes a JSON-encoded operational package with the
// opkg canonical checksum embedded. opkg.Validate re-canonicalises on
// read and re-checksums, so the on-disk JSON does not need to be
// byte-identical to opkg's canonical encoding; it just needs to
// round-trip the same struct values. Indented JSON keeps the example
// diff-friendly.
func writeExamplePackage(path string, pkg *opkg.Package, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return nil
	}
	pkg.Provenance.ChecksumSHA256 = opkg.Checksum(pkg)
	body, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

const readmeBody = `# Scenario preparation workspace (DRAFT)

This workspace was created by ` + "`scenario-prep init`" + ` as a DRAFT.
DRAFT files will not pass validation as launch-ready artefacts; you must
replace the placeholders with your own user-supplied data.

## What you will supply

1. A canonical catalogue (replace ` + "`catalogue.template.json`" + `):
   - one entry per selected state with ` + "`state_code`" + `, ` + "`name`" + `, ` + "`languages`" + `, ` + "`scenarios`" + `
   - 10–15 selected states is the launch target
   - 2–3 sourced scenarios per state
   - historical events must reference an evidence ` + "`source_ref`" + ` you control
2. Per-scenario operational packages (replace ` + "`package.template.json`" + `):
   - one JSON file per scenario, validated by the existing ` + "`opkg`" + ` boundary
   - zones, routes, instructions, contacts, policy
   - the ` + "`provenance.evidence_class`" + ` must reflect your real data origin
3. An input index (replace ` + "`index.template.json`" + `):
   - workspace-relative paths to the catalogue and each package
4. A reviewer-supplied ` + "`exercise_clock`" + ` (RFC 3339) so the run is anchored

## What this tool does

- ` + "`validate`" + ` — structural check; no files written
- ` + "`report`" + ` — JSON + Markdown review reports
- ` + "`bundle`" + ` — deterministic handoff directory for review

## What this tool does NOT do

- sign packages or claim P5 offline-client compatibility
- contact any government system, model, or database
- infer routes, zones, languages, or scenarios the user did not supply
- call signature verification trusted (it reports UNVERIFIED)
`

const catalogueTemplate = `{
  "_comment": "DRAFT catalogue skeleton. Replace placeholders with your own user-supplied data; the tool will not choose states for you. Items prefixed with REPLACE_ME will fail validation.",
  "catalogue_version": 1,
  "states": [
    {
      "state_code": "REPLACE_ME_STATE_CODE",
      "name": "REPLACE_ME_STATE_NAME",
      "languages": [
        { "language": "REPLACE_ME_LANG_BCP47", "asr_supported": false, "tts_supported": false }
      ],
      "scenarios": [
        {
          "scenario_id": "REPLACE_ME_SCENARIO_ID",
          "state": "REPLACE_ME_STATE_CODE",
          "evidence": "SYNTHETIC_EXERCISE",
          "historical_event_id": "",
          "exercise_time": "REPLACE_ME_RFC3339"
        }
      ]
    }
  ],
  "historical_events": []
}
`

const packageTemplate = `{
  "_comment": "DRAFT operational-package skeleton. opkg.Validate requires a checksum; the tool will refuse to run on placeholder data. REPLACE_ME values fail validation.",
  "provenance": {
    "dataset_id": "REPLACE_ME_DATASET_ID",
    "evidence_class": "SYNTHETIC_DEMO",
    "version": 1,
    "jurisdiction": "REPLACE_ME_STATE_CODE",
    "authority": "REPLACE_ME_AUTHORITY",
    "effective_at": "REPLACE_ME_RFC3339",
    "expires_at": "REPLACE_ME_RFC3339",
    "checksum_sha256": "REPLACE_ME_HEX_64"
  },
  "alert": { "identifier": "REPLACE_ME_ALERT_ID" },
  "red_zones": [],
  "safe_zones": [],
  "approved_routes": [],
  "instruction_assets": [],
  "facilities": [],
  "allocation_policy": { "order": [] },
  "emergency_contacts": []
}
`

const indexTemplate = `{
  "_comment": "DRAFT input index. Map scenario_id -> workspace-relative package path. Use --exercise-clock to anchor the run.",
  "catalogue": "catalogue.json",
  "scenarios": {
    "REPLACE_ME_SCENARIO_ID": "REPLACE_ME_PACKAGE_REL_PATH"
  },
  "exercise_clock": "REPLACE_ME_RFC3339",
  "notes": "DRAFT; replace before validate."
}
`

const exampleCatalogue = `{
  "catalogue_version": 1,
  "historical_events": [
    {
      "event_id": "EV-EXAMPLE-2018",
      "state": "EX",
      "kind": "flood",
      "occurred": "2018-08",
      "source_ref": "example:user-supplied-2018-flood-report"
    }
  ],
  "states": [
    {
      "state_code": "EX",
      "name": "Example State (SYNTHETIC)",
      "languages": [
        { "language": "en-IN", "asr_supported": false, "tts_supported": false },
        { "language": "ex-IN", "asr_supported": false, "tts_supported": false }
      ],
      "scenarios": [
        {
          "scenario_id": "SC-EX-2018-FLOOD",
          "state": "EX",
          "evidence": "HISTORICAL_EVENT",
          "historical_event_id": "EV-EXAMPLE-2018",
          "exercise_time": "2026-09-12T04:00:00Z"
        },
        {
          "scenario_id": "SC-EX-SYNTH-1",
          "state": "EX",
          "evidence": "SYNTHETIC_EXERCISE",
          "exercise_time": "2026-09-12T04:00:00Z"
        }
      ]
    }
  ]
}
`

const exampleIndex = `{
  "catalogue": "examples/catalogue.json",
  "scenarios": {
    "SC-EX-2018-FLOOD": "examples/pkg-sc-hist-1.json",
    "SC-EX-SYNTH-1": "examples/pkg-sc-synth-1.json"
  },
  "exercise_clock": "2026-09-12T04:00:00Z",
  "notes": "SYNTHETIC EXAMPLE only; not a real event. The historical link references a user-supplied reference, not government authority."
}
`

// exampleSyntheticPackage builds a small SYNTHETIC_EXERCISE package.
func exampleSyntheticPackage(datasetID, jurisdiction string) *opkg.Package {
	cap1 := 100
	cap2 := 50
	expiry := 600
	walkIns := true
	pkg := &opkg.Package{
		Provenance: opkg.Provenance{
			DatasetID:     datasetID,
			EvidenceClass: opkg.EvidenceSynthetic,
			Version:       1,
			Jurisdiction:  jurisdiction,
			Authority:     "example.synthetic.ddma",
			EffectiveAt:   "2026-09-12T04:00:00Z",
			ExpiresAt:     "2026-09-13T04:00:00Z",
		},
		Alert: opkg.Alert{Identifier: "ALERT-EX-SYNTH-1"},
		RedZones: []opkg.Zone{
			{ID: "RZ-EX-01"},
		},
		SafeZones: []opkg.Zone{
			{ID: "SZ-EX-01", Capacity: &cap1, Location: []float64{76.5, 11.5}, Status: opkg.ZoneOpen},
			{ID: "SZ-EX-02", Capacity: &cap2, Location: []float64{76.6, 11.6}, Status: opkg.ZoneOpen},
		},
		ApprovedRoutes: []opkg.Route{
			{
				ID:           "RT-EX-01",
				FromZoneID:   "RZ-EX-01",
				ToSafeZoneID: "SZ-EX-01",
				Approval:     opkg.ApprovalSynthetic,
				Mode:         opkg.ModeFoot,
				Geometry:     []byte(`{"type":"LineString","coordinates":[[76.0,11.4],[76.5,11.5]]}`),
			},
		},
		Instructions: []opkg.InstructionAsset{
			{ID: "INS-EX-EN", Language: "en-IN"},
			{ID: "INS-EX-LOCAL", Language: "ex-IN"},
		},
		Facilities: []opkg.Facility{
			{ID: "FAC-EX-01", SafeZone: "SZ-EX-01"},
		},
		Policy: opkg.AllocationPolicy{
			Order:                   []string{"SZ-EX-01", "SZ-EX-02"},
			ReservationExpirySeconds: &expiry,
			AllowWalkIns:             &walkIns,
		},
		Contacts: []opkg.EmergencyContact{
			{Name: "Synthetic Operations", Number: "112"},
		},
	}
	return pkg
}

// exampleHistoricalPackage builds a small HISTORICAL_EVIDENCE package
// reusing the synthetic geometry. The historical event provenance label
// stays on the package; the geometry itself is still SYNTHETIC and is not
// upgraded because a real past event is linked.
func exampleHistoricalPackage(datasetID, jurisdiction string) *opkg.Package {
	pkg := exampleSyntheticPackage(datasetID, jurisdiction)
	pkg.Provenance.EvidenceClass = opkg.EvidenceCaptured
	pkg.Alert = opkg.Alert{Identifier: "ALERT-EX-HIST-1"}
	return pkg
}