# Scenario preparation workspace (DRAFT)

This workspace was created by `scenario-prep init` as a DRAFT.
DRAFT files will not pass validation as launch-ready artefacts; you must
replace the placeholders with your own user-supplied data.

## What you will supply

1. A canonical catalogue (replace `catalogue.template.json`):
   - one entry per selected state with `state_code`, `name`, `languages`, `scenarios`
   - 10–15 selected states is the launch target
   - 2–3 sourced scenarios per state
   - historical events must reference an evidence `source_ref` you control
2. Per-scenario operational packages (replace `package.template.json`):
   - one JSON file per scenario, validated by the existing `opkg` boundary
   - zones, routes, instructions, contacts, policy
   - the `provenance.evidence_class` must reflect your real data origin
3. An input index (replace `index.template.json`):
   - workspace-relative paths to the catalogue and each package
4. A reviewer-supplied `exercise_clock` (RFC 3339) so the run is anchored

## What this tool does

- `validate` — structural check; no files written
- `report` — JSON + Markdown review reports
- `bundle` — deterministic handoff directory for review

## What this tool does NOT do

- sign packages or claim P5 offline-client compatibility
- contact any government system, model, or database
- infer routes, zones, languages, or scenarios the user did not supply
- call signature verification trusted (it reports UNVERIFIED)
