# Handoff: R05 — Safe scenario preparation and truthful demo data

Task / implementation status / acceptance status:
- Task: R05 (Data lane)
- Implementation status: VERIFIED
- Acceptance status: VERIFIED (all acceptance criteria met)

Base / branch / worktree / implementation commits:
- Base: 1347c2f (CLEAN)
- Branch: CLEAN
- Modified files: backend/internal/scenarioprep/{path.go, index.go, validate.go, bundle.go, prepare.go, prepare_test.go}, backend/cmd/scenario-prep/report.go

Observable changes and key paths:
- path.go: safeResolve now resolves every parent and leaf path component against realRoot, rejecting any symlink that escapes the workspace (via ErrUnsafeSymlink).
- path.go: added readBoundedFile to bound reads (Lstat check + Size limit check + LimitReader) before memory allocation or decoding, eliminating unbounded read vectors.
- path.go: added destinationExists to detect existing files, directories, and symlinks.
- index.go: LoadIndex bounds index read to 1 MB (indexLimits.MaxBytes) before decoding, mapping ErrBoundedRead to contracts.ErrBodyTooLarge.
- index.go: resolveReferences uses safeResolve (symlink escape detection) and readBoundedFile (4 MB limit) before package hashing.
- validate.go: catalogue and package reading now enforce symlink checks and bounded reads.
- bundle.go: Bundle and writeRejection now refuse to overwrite existing destination directories/files (checked via destinationExists). Bundle reads files via readBoundedFile.
- cmd/scenario-prep/report.go: fileExistsCheck updated to use os.Lstat, refusing overwrite on existing files or directories.
- prepare_test.go: added tests covering parent/leaf symlink escapes, oversized index/package, destination overwrite prevention on directories, output inside input, and incomplete draft handling (14/14 tests PASS).

Acceptance checklist: requirement -> test/run -> result:
- Parent/leaf symlink escape -> TestParentSymlinkEscapePackage, TestLeafSymlinkEscapePackage, TestParentSymlinkEscapeCatalogue, CLI E2E -> PASS (UNSAFE_SYMLINK finding, status INVALID, exit 2)
- Oversized index rejected before unbounded read -> TestRejectsOversizedIndex -> PASS (rejected with size limit error)
- Oversized package rejected before unbounded read -> TestRejectsOversizedPackage -> PASS (MALFORMED_PACKAGE_FILE finding, status INVALID)
- Path traversal (.., ~, absolute) -> TestRejectsPathEscape -> PASS (UNSAFE_REFERENCE_PATH finding, status INVALID)
- Existing output preserved -> TestBundleRefusesOverwrite, TestBundleRefusesExistingDirectory, CLI E2E -> PASS (refused with ErrIO / exit 4)
- Output inside workspace refused -> TestBundleRefusesOutputInsideWorkspace -> PASS (ErrUnsafeLayout)
- Malformed/duplicate-key inputs -> TestRejectsDuplicateIndexKeys -> PASS (strict decode failure)
- Stable bundle hashes -> TestBundleDeterministicRepeat -> PASS (byte-identical manifest across runs)
- Missing data posture -> TestIncompleteHandling, CLI E2E -> PASS (status INCOMPLETE, exit 3, draft bundle with --allow-draft, rejected bundle without --allow-draft)
- Full roundtrip init/validate/report/bundle -> CLI E2E test script -> PASS (all subcommands succeed with documented exit codes)

Commands, versions, environment, actual exit codes and skipped checks:
- go version: go1.24.0 darwin/arm64
- gofmt -l on changed files: exit 0 (clean)
- go vet ./internal/scenarioprep/... ./cmd/scenario-prep/...: exit 0
- go test -count=1 -v ./internal/scenarioprep/...: exit 0 (14/14 PASS)
- scenario-prep CLI e2e test script: exit 0 (all 8 test stages passed)
- Skipped checks: none.

Real vs fake evidence, device/model/data revisions when relevant:
- Offline data preparation CLI verified on local filesystem with real file IO and symlinks.
- Deterministic synthetic exercise fixtures used; signature verification truthfully reported as UNVERIFIED.

Remaining defect or external blocker, exact owner/input needed:
- No tool defects remaining.
- External gate O01 (real 10-15 states catalogue) remains open for curator/user data.

Shared-contract changes required (or none):
- None. Only scenarioprep package and cmd/scenario-prep tooling files touched.

Next eligible task and integration order:
- R06 (reproducible local deployment) or integration with R01/R02 once available.

Owned resources cleaned/preserved:
- Temporary test directories and CLI binaries cleaned.
