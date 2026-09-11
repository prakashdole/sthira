# UI contrast refinement

- [x] Inspect current localhost rendering and identify contrast/hierarchy problems.
- [x] Refine the imagery viewer palette, spacing, and responsive behavior without changing the demo flow.
- [x] Verify JavaScript, backend tests, and desktop/mobile rendering.

## Review

Replaced the competing cream/yellow/green treatments with a restrained slate-teal
operations palette, improved semantic status contrast, simplified borders and
shadows, and restored a single-column viewer below 980px. Visual QA passed in
the localhost browser. `node --check frontend/app.js` passed and the full backend
suite passed: 183 tests, 2 existing deprecation warnings.

## Pastel palette pass

- [x] Tokenize a pastel-blue and pastel-green palette.
- [x] Apply it without weakening semantic status contrast.
- [x] Verify localhost rendering and focused checks.

Pastel blue now carries the page chrome and informational surfaces; pastel green
carries prototype and evidence surfaces. Dark blue-green ink and a deeper green
CTA preserve readable contrast. Browser visual QA passed, `tokens.css` returned
HTTP 200, JavaScript syntax passed, and all 183 backend tests passed.

## Teal shell refinement

- [x] Restore the deep teal application shell.
- [x] Shift the calm workspace and evidence surfaces toward pastel blue.
- [x] Keep pastel green for secondary status cues and verify the result.

Visual QA confirmed the teal shell, pastel-blue workspace, and restrained green
status accent work together at the localhost viewport. The token stylesheet
returned HTTP 200, JavaScript syntax passed, and `git diff --check` found no
whitespace errors.

## Triple verification demo

- [x] Add satellite, ground, and radar/elevation stage data to the demo API.
- [x] Show the three-stage method and per-site stage outcomes in the viewer.
- [x] Verify API behavior, frontend syntax, and localhost rendering.

The API now returns a transparent three-stage verification trail for every site.
The frontend shows the method before execution and the three outcomes per site,
with underlying criteria available on demand. Radar/elevation values are labelled
as synthetic demo fixtures. Visual QA passed and all 183 tests passed.

## Animated map screening

- [x] Pulse the historical high-hazard area on initial page load.
- [x] Hide candidate markers until screening begins.
- [x] Reveal each site sequentially with checking and final-result callouts.
- [x] Verify reduced-motion behavior, JavaScript, tests, and localhost rendering.

The map now starts with only the pulsing red historical hazard zone. Running the
screening reveals Elstone, Nedumbala and Kottapadi sequentially, shows a temporary
three-source checking callout, and resolves each to Pass, Verify or Fail. Reduced
motion falls back to immediate static visibility. Browser QA and all 183 tests passed.

## Voice-guide interface

- [x] Add an original animated Sthira companion as the voice-chat entry point.
- [x] Add a compact multilingual Sarvam-ready voice panel.
- [x] Add an honest listening-state preview without requesting microphone access.
- [x] Verify keyboard behavior, reduced motion, syntax, and localhost rendering.

The original Sthira Guide companion opens a floating English/Malayalam/Hindi
voice panel. Its listening preview animates without requesting microphone access
or claiming a Sarvam connection, and the selected language persists across states.
Browser QA passed, JavaScript syntax passed, and all 183 tests passed.

## Full-page language and UI polish

- [x] Create a feature branch before committing.
- [x] Extend language selection to every visible interface string.
- [x] Recompose the map and evidence areas with better spacing and hierarchy.
- [x] Refine the white/green palette while retaining teal and pastel blue.
- [x] Verify syntax, tests, responsive rendering, and commit the intended diff.

The page now uses one language selector for the shell, map, workflow, screening
results, loading states, warnings and voice guide. The layout is map-first, with
the verification method below the imagery and result cards using the full page
width. Visual browser checks passed in English and Malayalam; JavaScript syntax
passed, `git diff --check` passed, and the configured virtual environment passed
all 183 tests with 2 existing deprecation warnings.

## ASL access option

- [x] Add ASL as the fourth interface option.
- [x] Keep readable English copy in ASL mode.
- [x] Show an honest signed-video placeholder instead of claiming text translation.
- [x] Verify the interaction and focused checks.

ASL now appears as the fourth selector. It activates a prominent interpretation
panel before the satellite scene, retains readable English text, and states that
interpreter-recorded clips are not connected yet. Browser interaction, visual
layout, JavaScript syntax, and whitespace checks passed.

## Candidate marker visibility

- [x] Restore all three site markers on initial map load.
- [x] Verify Nedumbala and Kottapadi visually before screening.

The three marker anchors now remain visible within the center-cropped map at
initial load. Browser QA confirmed Elstone, Nedumbala and Kottapadi together.
