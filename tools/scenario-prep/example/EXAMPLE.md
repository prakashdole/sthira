# scenario-prep example walkthrough

This example shows the real CLI workflow from `init` through `bundle`
using a small clearly synthetic workspace. It is intentionally below the
10-state launch minimum; running `bundle` without `--allow-draft`
produces a rejection summary.

## What is in this directory

```
example/
  README.md                    # the init-printed workspace README (DRAFT)
  catalogue.template.json      # DRAFT placeholder skeleton
  package.template.json        # DRAFT placeholder skeleton
  index.template.json          # DRAFT placeholder skeleton
  examples/
    catalogue.json             # synthetic 1-state catalogue
    index.json                 # maps scenarios -> packages
    pkg-sc-synth-1.json        # SYNTHETIC_EXERCISE opkg (computed checksum)
    pkg-sc-hist-1.json         # HISTORICAL_EVIDENCE opkg (computed checksum)
```

The example deliberately uses state code `EX` and language code `ex-IN`
so it cannot be mistaken for an authorised real region. The historical
event references a user-supplied source — not government authority.

## Run the example end to end

```sh
cd tools/scenario-prep/example

# 1. structural check
scenario-prep validate --workspace . --index examples/index.json

# 2. write a review report (creates ./reports/)
scenario-prep report --workspace . --index examples/index.json

# 3. publish a draft handoff bundle (--allow-draft because INCOMPLETE)
scenario-prep bundle --workspace . --index examples/index.json \
                    --output ../example-bundle --allow-draft
```

## Replace the DRAFT templates with real data

1. Copy `catalogue.template.json` to `catalogue.json` and fill in real
   states (10–15), languages, and 2–3 scenarios per state.
2. Build per-scenario opkg JSON files matching the scenario IDs.
3. Update `index.json` with the relative paths.
4. Re-run `validate`; status will move from `INCOMPLETE` to `READY` once
   enough states and scenarios are supplied.
5. Run `bundle` (without `--allow-draft` once the catalogue is
   launch-ready).

## What this example does not satisfy

- O01: it does not satisfy the approved demo state/district list.
- O05: it does not satisfy route authority approval.
- O07/O08/O11/O14: none of the open operational gates are closed.

The example is a workflow demonstration, not a launch artefact.