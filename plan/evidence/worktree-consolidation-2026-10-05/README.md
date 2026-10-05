# Worktree consolidation — 2026-10-05

## Primary checkout

`/Users/apple/Documents/Projects/MonitoringZ`, branch `CLEAN`.
At audit start: `abe3bb4565a7c84d37dcf6788e0d979505eb172d`.
Fetched origin; neither `origin/CLEAN` nor `origin/main` had commits absent from this baseline.
The local branch literally named `main` was older and was not changed.
Existing untracked `loop.toml` and `tasks.json` were left untouched and unstaged.

## Integration and preservation

- Fast-forwarded CLEAN to `ced0df42152f817de0289617c1fc4fd13f4e2a6d` (loop/integration).
- T002: route focus preserves manually chosen camera pitch in 2D; tests included.
- T003: mock-worker handler extraction and scenario contract tests.
- T005: health omits checksum-less models/artifacts; unconditional tests and positive control.
- T001 (`0541a91`) and T004 (`636efff`) are alternate implementations of the same health fix. T005 retains their substantive guards and zero-entry assertions, adds a positive control, and runs the existing defect checks without environment gates. Their exact changed file versions are preserved under historical/ rather than layering conflicting implementations.
- Lane 1 (`2caa85a`): map retry fix is patch-equivalent to `23f77aa` in CLEAN; its earlier draft/modal change was intentionally split/corrected by `329633b` and `b595926`, followed by the stronger map-load gate `2beaaa6`. Historical code and reports are preserved without replacing these newer fixes.
- Lane 2 (`6a68524`): preserves four older commits of reports and presenter documentation. Current presenter/browser evidence and tracker contain later corrections; all seven changed historical files are preserved without overwriting current documents or accepting historical claims as current verification.

`manifest.json` records the source commit, original path, and SHA-256 for all 14 exact historical file snapshots. These are reference material, not active source or current acceptance evidence.
All original branch references and their complete commit history remain in the primary repository. Removing a linked checkout does not delete its branch.

## Cleanup boundary

Eight extra linked worktrees were audited. All had clean tracked state and no non-ignored untracked files. Only three contained ignored files: frontend node_modules and, in two cases, dist. These are rebuildable dependency/build output; no credentials, model assets, unique ignored reports, or local configuration were found in the extra worktrees. No running process referenced the extra paths during the audit.
Remove only those eight registered worktrees after final checks and the preservation commit; keep the primary checkout and unrelated directories.

## Verification

- Mock-worker module: go vet and uncached tests passed.
- Middleworker module: go vet and uncached full module tests passed.
- Frontend: 97 tests passed, TypeScript check and production build passed; Vite emitted its large-chunk warning.
- Full backend: build, vet, and uncached full tests passed. Middleworker: uncached race-detector full module tests passed.
- Snapshot bytes were hash-verified against their source commits.
- No .txt changes and no push. This is consolidation verification, not a fresh live-government/model or browser acceptance run.
