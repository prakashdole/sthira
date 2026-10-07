# Astra workstation starter — open-data pivot

Prepared 2026-10-07. This is the portable handoff for a new Codex chat on the receiving computer. It records a discussion and a workflow, not approval to implement every proposal below.

## Start here

You are Astra, coordinating Sthira/MonitoringZ on this computer. The user is moving work from a Mac whose chats are not available here. Continue from this repository and this handoff; do not start the application again from scratch.

- Repository: https://github.com/prakashdole/sthira.git
- Shared checkpoint branch: uppercase `CLEAN`, never `main`.
- Mac product checkpoint before this handoff: `82c767d15501d748eb35244c94437e4033bf2411`.
- The transfer commit adds this handoff and preserves the existing `loop.toml` and `tasks.json`. Obtain its full SHA from the user's transfer message or Git history and verify it before work. Do not mistake the pre-handoff SHA for the transfer SHA.
- The user wants Astra as coordinator and has described a pool of 16 development workers. Discover the actual local harness, configured models, permissions and concurrency limits. Sixteen is a requested pool size, not evidence that 16 concurrent agents are available or appropriate.
- Development workers and the deployed GLM inference service are separate systems.

Your first assignment is to verify receipt, inspect current implementation and planning contradictions, prepare the revised plan and worker workflow, then discuss the unresolved product decisions with the user. Do not begin broad product implementation, launch the old task queue, buy services, download models or deploy the app during this first assignment.

## Confirmed user direction

1. The product will operate independently, without a government partnership or privileged government integration.
2. Prefer publicly accessible, legally reusable sources that do not require access tokens, social-platform login or private accounts. News is a proposed major source for discovering disaster incidents.
3. Voice-first interaction is no longer wanted. Essential features must work through touch and text without server ASR/TTS.
4. The user wants to host the app with as little ongoing compute and operational cost as practical while preserving useful features.
5. Relocation remains central. The latest checked PRD concerns immediate shelter and 7–30 day temporary stays. The user has not yet answered whether permanent relocation returns to scope.
6. The user reports an existing deployed "GLM 5.3 flash" service producing approximately 150 tokens/second at maximum effort. This is a user-reported deployment, not a verified model identifier, API contract, language evaluation or concurrency benchmark. Discover its real interface and access arrangements before integration; keep credentials outside Git and chat reports.
7. The user wants the direction discussed before deciding the next implementation step.
8. Hosting the app is confirmed intent. Public release of the source code, a specific licence, commercial operation and a hosting provider are not decided.

## Recommendations discussed but not yet approved

- Continue using openly published government information where access and reuse allow it. No partnership does not necessarily mean excluding public IMD/SDMA/district bulletins. Ask if the user means to exclude these too.
- Focus first on Kerala, floods and landslides, and a few districts; retain broader coverage as a later expansion.
- Prefer a touch-first installable web app for the initial low-cost release. The repository also records a KMP/native mobile decision: a PWA is a new proposal, not permission to silently replace the existing decision or discard mobile work.
- Use GLM for background extraction, translation, duplicate detection and source-backed summaries. Process each changed incident once and serve cached results to users. Do not make ordinary navigation or public information depend on an LLM call.
- Retain the Go backend, PostgreSQL/PostGIS, existing durable state and offline safeguards where appropriate. Avoid a rewrite, a new microservice fleet or an inference call per interaction.
- Use modest regional maps and routing; avoid making continuous imagery processing, large satellite downloads, live voice generation or 3D rendering prerequisites.
- Support public incident/destination information independently of participating-facility booking. Confirmed reservations require current operator acceptance and authoritative capacity state.

## Source findings from the Mac discussion

These are a starting shortlist, not activated production integrations. Access checks were performed on 2026-10-07; endpoint reachability alone does not prove freshness, completeness, reuse rights or reliability.

| Source | Evidence and intended role | Qualification still needed |
| --- | --- | --- |
| GDELT | Official DOC API and terms pages were read. Candidate multilingual news discovery. GDELT datasets permit reuse with citation. https://blog.gdeltproject.org/gdelt-doc-2-0-api-debuts/ ; https://www.gdeltproject.org/about.html#termsofuse | Live query behavior, limits, local-language recall and event delay. Dataset rights do not grant republication rights to the underlying articles. |
| GDACS | Token-free RSS returned HTTP 200 and parsed items. https://www.gdacs.org/xml/rss.xml ; https://data.gdacs.org/feed_reference.aspx | India/local-event coverage, timestamps, revisions and reuse conditions. Major-event coverage does not establish village-level coverage. |
| Google News RSS | Token-free query returned HTTP 200 and parsed items. https://news.google.com/rss/search?q=Kerala+flood+landslide&hl=en-IN&gl=IN&ceid=IN:en | Supplementary discovery only; undocumented service stability and reuse terms. Search results may contain historical stories. |
| Publisher feeds | The Hindu publishes a feed directory at https://www.thehindu.com/rssfeeds/ | Qualify publishers individually. Public availability is not unrestricted use. |
| Onmanorama | Its https://www.onmanorama.com/rss.html terms restrict feeds to personal use and require consent for commercial use. | Do not treat this as an approved hosted-app source. |
| Public Indian bulletins | Existing source register lists IMD, SDMA/DDMA and other candidates. KSDMA content was retrieved with an upstream 429 warning. | No reliable automated Indian bulletin integration was established in this pass. Check exact feed/page, access terms, update/cancel semantics and failure behavior. |
| Open-Meteo | Token-free forecast call returned HTTP 200 and parsed data. https://open-meteo.com/en/pricing | Free hosted service is non-commercial, rate-limited and has no uptime guarantee. Weather forecasts do not establish observed street-level flooding. |
| OpenStreetMap | Candidate base geography. https://www.openstreetmap.org/copyright | Attribution/ODbL and selected hosting/offline rights. https://operations.osmfoundation.org/policies/tiles/ prohibits bulk/offline use of the public raster tile service. Do not use public Nominatim as an unlimited production geocoder. |
| ReliefWeb | https://apidoc.reliefweb.int/ documents pre-approved appname requirements effective 2025-11-01. | Does not meet a strict zero-registration requirement. |
| USGS earthquakes | Token-free GeoJSON feed returned HTTP 200 and parsed features. | Optional future hazard coverage, not a reason to expand the initial flood/landslide scope. |

For each proposed operational source record the retrieval method, rights, auth requirement, quotas, coverage, observed delay, timestamps, retention, attribution, update/cancellation handling and failure behavior. Keep unqualified sources explicit. Avoid scraping full articles or retaining them without a suitable basis.

## Product boundaries that survive the pivot

- Separate a reported incident, an observation, a forecast and an instruction. Publication time, event time and ingestion time are different fields.
- Preserve source links and supporting evidence. A source may be uncertain, stale, contradictory or retracted.
- Copied news stories are not independent confirmations. GLM confidence is not a calibrated probability of safety.
- Resolve locations against real geographical records, with ambiguity and spatial precision represented. A district-level article cannot create an exact hazard polygon.
- No reports found does not mean no hazard. Historical data and synthetic exercises must remain visibly distinct from current operational data.
- Mapped buildings are not confirmed shelters. Facility type does not establish opening status, accommodation suitability or available beds.
- Road geometry supports candidate routes, not current emergency passability. Known closure information and local verification matter; a lack of closure reports does not certify a route.
- Rank destinations only after applicable eligibility checks. Unknown capacity stays unknown; show contact-to-confirm rather than a confirmed reservation.
- Reservation, arrival, cancellation, departure and transfer must preserve party size, idempotency, concurrency and capacity invariants. Self-reported arrival is not independent welfare confirmation.
- Offline information must expose age and expiry; absence of connectivity cannot create a new confirmed booking or promise current road status.
- Public-source and operator authority must be represented honestly. Redesign the government-specific trust policy deliberately; do not bypass it globally or relabel media reports as government approval.
- Permanent relocation, if requested later, needs separate land tenure, availability, consent, livelihood and field-review requirements. Public maps or satellite screening cannot prove title or suitability.

## Receive the checkpoint safely

Use this computer's real local path. Do not assume `/Users/apple/...` exists. If this is Windows, inspect the actual shell and any available WSL environment before selecting commands. Do not install or assume WSL automatically.

For a new clone, run in an appropriate parent directory where `sthira` does not already exist:

```sh
git clone --branch CLEAN --single-branch https://github.com/prakashdole/sthira.git sthira
cd sthira
git status --short --branch
git rev-parse HEAD
```

For an existing clone, first inspect its remote, branch, working tree and local commits. If clean and compatible, fetch, switch to `CLEAN` and fast-forward only. Never run pull blindly into a dirty or divergent checkout; preserve existing work and use a separate clone when appropriate. No reset, force checkout, automatic stash, force push or history rewrite.

Compare the full received SHA with the transfer SHA. If GitHub has advanced, establish whether the transfer commit is an ancestor and inspect the added commits rather than declaring them equivalent. Record the verified baseline SHA.

The Git transfer contains committed files and reachable history. It does not transfer ignored `.env`, `.ai`, `.loop`, virtual environments, node_modules, generated output, model weights, databases, running processes, local Codex settings or chat history. Do not claim these exist on the workstation. Rebuild ordinary dependencies from manifests; obtain necessary secrets through the local secret mechanism without printing them. The user must separately arrange any required private/local data transfer.

The root `loop.toml` and `tasks.json` are historical records preserved at the user's request, NOT a launch instruction:

- `loop.toml` defines four MiniMax worker slots, not 16 Astra workers.
- Its integration branch is `loop/integration`; its check/setup commands are empty.
- All six listed root-level planning paths were missing on the Mac; current documents live under `plan/`.
- `tasks.json` contains old IDs, Mac paths, PIDs, sessions and statuses. Its five entries have no recorded `verified_sha`.
- Never resume/kill processes using these recorded PIDs, reuse those task IDs as new work, or treat `done` as proof of acceptance.
- Preserve the old state. Prepare a fresh run/config only after reading the installed harness documentation and checking actual model identifiers, supported options and paths. No guessed CLI flags or copied credentials.

## Inspect before planning

Read applicable AGENTS.md instructions and `tasks/lessons.md`. Then inspect `README.md`, the top/current status of `plan/prompt.md`, `plan/TODO.md`, `plan/prd.md`, `plan/feature.md`, `plan/architecture.md`, `plan/tech-stack.md`, `plan/source-register.md`, `plan/decisions.md`, `plan/open-decisions.md` and `plan/phases.md` selectively. Read affected implementation paths rather than all historical reports.

Existing plans contain government-only and voice-first requirements and old phase gates. Treat them as the previous baseline and identify the exact proposed changes. Do not continue old voice tasks merely because they are queued. Preserve decisions and evidence historically; do not erase failures or describe this handoff as acceptance of old code.

Verified layout on the Mac:

- Active API: Go in `backend/`; `backend/contracts/openapi.yaml` describes `/api/v3`.
- Current web UI: `frontend/v2/`, TypeScript/Vite/MapLibre.
- Database: PostgreSQL/PostGIS; inspect current migrations and required versions.
- Voice/model worker modules have their own `go.mod`; a test at `backend/` does not automatically exercise every nested module.
- Legacy Python remains reference/model-adapter code, not the place to restart the product API.
- Existing mobile work is under `mobile/`; review before proposing a platform change.

Inspect actual manifests and tool availability. `backend/go.mod` specified Go 1.27.1 on the Mac. The frontend has a package lock and `test`/`build` scripts. Use installed/approved runtimes matching the project, and report unavailable prerequisites without silently changing versions.

Establish a workstation baseline using existing checks. Typical entry points, after prerequisites are available:

```sh
# Within backend/
go vet ./...
go build ./...
go test ./...

# Within frontend/v2/
npm ci --ignore-scripts
npm test
npm run build
```

These are starting checks, not a complete acceptance suite. Inspect the Makefile, module boundaries, DB test setup and browser evidence scripts. DB tests need owned disposable databases; never reuse production or unrelated databases. A skipped DB/browser check is NOT_RUN, not PASS. Use bounded logs, preserve actual exit status and avoid repeating unchanged successful suites. Do not make the baseline depend on downloading obsolete voice models.

## Astra and worker workflow

1. Astra owns planning, task dependencies, worktree creation, integration, final verification and the shared checkpoint. Workers never edit the coordinator's checkout.
2. Use at most the actual supported concurrency. Begin with a small useful batch, prove the harness works, then increase only when tasks are independent and machine resources allow it. A pool of 16 does not require 16 simultaneous jobs.
3. Initial useful work is bounded read-only investigation: current feature/implementation inventory; source qualification; low-compute architecture/reuse assessment; and acceptance/failure-mode review. Do not give multiple workers the same whole-repository audit.
4. Once the user approves a revised direction, create one `codex/` branch and isolated worktree per editing task, from an explicit baseline commit. Keep the shared checkpoint on `CLEAN`. Use a separate `codex/` integration branch for candidate changes until reviewed; only the coordinator advances `CLEAN`.
5. Assign each worker a task ID, goal, exact base SHA, owned files, dependencies, observable acceptance criteria, checks, prohibited changes and report path. Overlapping ownership requires sequencing. Never have 16 workers independently edit `main.ts`, common contracts or global planning files.
6. Workers may commit verified stages in their own branches; only Astra integrates/publishes the shared result. No amend/reset/rebase/force push, destructive cleanup or edits to another worker's state. Workers cannot declare their own work accepted.
7. Every worker report records full commit SHA, actual changed paths, check commands and exit statuses, pass/fail/skip, reproducible evidence and remaining limitations. Put sanitized reports under `plan/worker-reports/open-data-pivot/<task-id>.md`. Do not store credentials, real household data, audio or precise personal locations there.
8. Astra reviews the actual diff, checks ownership/scope, exercises the claimed boundary and records ACCEPTED, REJECTED, NEEDS_CHANGES or BLOCKED with reasons. Evidence applies only to the checked revision. Green unit tests alone do not prove an HTTP/DB/browser/data-source flow.
9. Integrate compatible accepted changes sequentially, inspect the combined diff and run affected integration checks. Never merge merely because all workers say done. Make stage commits with concise intent-focused messages.
10. Use one owned synthetic app stack for browser verification, serialized/resettable sessions where shared state exists, and separate ports/databases for concurrent tests. Track process ownership; never kill by port or stale PID.
11. Keep a compact coordinator handoff under `plan/worker-reports/open-data-pivot/coordinator.md`: baseline/integrated SHA, task disposition table, decisions, checks, blockers and next step. Shared planning docs have one writer. This file and Git commits carry context between chats and computers.
12. The receiving coordinator remains the only product writer across machines during the run. The Mac may review published checkpoints. A second chat must not create competing fixes without an agreed task boundary.
13. GitHub is the transfer mechanism, not live shared execution or chat sync. Publish accepted checkpoint branches only with user authorization. For this initial planning assignment, create local commits for reviewed handoff/planning work but do not assume authorization for ongoing pushes, PRs, merges to `CLEAN`, deployment or paid services. Obtain an explicit ongoing checkpoint-publication instruction if needed.
14. After three consecutive unsuccessful attempts on the same issue without material new evidence, stop that issue, report the evidence and continue independent work. Do not cosmetically rephrase the same task to evade the limit.

Official Codex worktree guidance: https://developers.openai.com/codex/app/worktrees . Worktrees isolate working files while sharing Git metadata; each checked-out branch needs its own worktree. Native subagent support and limits must be discovered on this computer. If unavailable, give the user separate worktree-scoped worker prompts instead of inventing orchestration commands or claiming workers were launched.

## First response and stop condition

After inspection, give the user:

1. The received repository/branch/full SHA, working-tree state and prerequisite check results.
2. What currently works, what is only mocked/reported, and what remains unverified. Cite files/commits and executed checks.
3. A proposed keep/change/defer feature map and the precise old government/voice/mobile assumptions that need revision.
4. A source shortlist with actual access/reuse/coverage limitations and a proposed pilot validation for incident recall, delay, false positives and location errors.
5. A simple architecture/cost plan that reuses existing code and makes core information available when GLM fails. Separate model inference, maps/bandwidth, database, availability and human-operator costs; no unsupported hosting-price or throughput promises.
6. A bounded worker task/dependency plan with non-overlapping ownership and integration gates, including how results return to the Mac by exact commit.
7. Only the material unresolved questions: temporary versus permanent relocation; whether public government data is allowed; initial geography/languages; commercial/free-service constraints and budget; and whether facility operators will participate.

Record confirmed direction, proposals and open decisions separately. Prepare a reviewable planning delta; do not mark proposals accepted or rewrite the previous PRD wholesale before the user decides. Stop before product implementation or live deployment. The immediate success criterion is a verified receiving checkpoint and a clear, reviewable plan for the pivot, not a launched 16-worker implementation run.
