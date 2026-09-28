# Demo day — one-page rehearsal sheet

**File:** `plan/evidence/demo-day-sheet.md` · **Baseline:** `2d48319` (branch `CLEAN`) · **Unstaged.** Labels: **SOURCE** = code/commit read · **OBSERVED(what)** = a run/command someone did · **PENDING** = planned, not done · **NOT_RUN** = never exercised. Every screen string below is copied verbatim from `frontend/v2/src/i18n.ts` (grepped, not retyped) — read them there; never retype a translation.

## (a) Bring-up — `./tools/demo/demo.sh` (SOURCE `b15cc4a`)

```sh
./tools/demo/demo.sh start                       # builds this tree, fresh throwaway DB, waits for READY
./tools/demo/demo.sh status                      # per-component pid/port/URL, database, /health/ready
./tools/demo/demo.sh stop                        # kills only this run's pids, drops only its DB
```
Default ports, 127.0.0.1 only: mock workers **18880**, exercise API **18881**, UI **18882**. Open **http://127.0.0.1:18882**.
Scenario: `destination-choice` (the default; others: `default`, `silent-zoom`, `arrival-confirm`, `clarify`, `data-unavailable`, `worker-failure`). Logs/state, no tokens or DSNs: `/tmp/sthira-demo/run/`. Leave `STHIRA_EXERCISE_SEED_COORDLESS=1` off unless rehearsing the coordinate-less rule (it adds a second facility `FACDEMO-2`).
OBSERVED(`curl`, live `destination-choice` run): `/health/ready` = `data.status READY`, `models` detail `all 3 worker stages ready`, `data_version PKGDEMO-1:1` — that detail string is from `readiness.go`, not i18n.

## (b) The 5-minute script — click, then say (plan; the browser walkthrough itself is NOT_RUN on this baseline)

| Time | Click | Expect on screen (verbatim i18n) | Say |
|---|---|---|---|
| 0:00 | open the UI | `Loading the map...` | "This is a synthetic exercise, not a live alert." |
| 0:20 | `Begin` → pick EN → `Continue` | `Choose the language you speak` | "Language is chosen by the citizen, never guessed." |
| 0:40 | `Use my location` or `Continue without location`, then `Start guidance` | map with red/relocation overlays | "Location is never sent to the server; the phone uses it only for the near-destination prompt." |
| 1:00 | toggle `3D view`, `Red zones`, `Relocation zones` | toggles change locally | "These are local map operations — no server call." |
| 1:40 | voice console → `Show my route` (or type in `Example: show my route` → `Run command`) | result text = the backend template `Destination choices are displayed on screen.` (`registry.go:116`, `SyntheticOnly: true` — it is **not** an i18n string) | "The model proposes; the Go server validates before the map moves." |
| 2:10 | tap the `Demo Safe Facility (SZDEMO-1)` chip → `Start safe route` | `Reserving...` → `Route active` | "Pending state first; the badge only after a 201." |
| 2:40 | in the arrival dialog (heading `Has everyone arrived safely?`) tap `Confirm arrival now` | `Arrival recorded with authority.` | "Arrival needs an explicit server acknowledgement." |
| 3:00 | `Source details` | `This screen uses a synthetic demo package. It is not an active government alert.` | "Every zone on this map is exercise data." |
| 3:30 | switch to HI or ML | labels switch; the same template returns in that language | "Hindi and Malayalam dialog wording is a draft awaiting native review." |
| 4:00 | — | — | "GPS shows where the phone is, not whether anyone is safe; arrival is the citizen's report plus a server check. Real government integration needs operational source agreements and an authorised inference environment." |

**Arrival needs the GPS near the synthetic shelter** (SOURCE: `main.ts` renders `arrival-open` only in `NEAR_DESTINATION`,
within 150 m). On a laptop elsewhere it never appears. Before `Use my location`, in Chrome open DevTools → More tools →
Sensors → Location → custom latitude `11.570`, longitude `76.105` (the exercise shelter, `mapData.json`), and say aloud that
the location is emulated. Safari and a real phone cannot do this; there, stop the script at `Route active`.

## (c) Say this about synthetic vs real

- **Synthetic, always in the default demo:** every zone, route, facility and capacity number — the approved-template registry marks these templates `SyntheticOnly: true` (SOURCE `registry.go:110-135`). The mock workers play fixed scripts and do not understand speech: the ASR returns the same transcript whatever you say, the middle stage returns the *configured scenario*, and audio is a pre-generated WAV. Nothing you say is transcribed or understood. SOURCE `tools/demo/README.md`.
- **Real (code exists, not exercised here):** the browser, the Go API, PostgreSQL persistence, idempotency, the server-side proposal validation, the `tel:112` dialler link.
- **NOT_RUN unless a paid session actually ran:** real ASR / Sarvam-30B / TTS inference, latency, speech quality, and all human language evaluation. If no paid session ran, say "the models were not run today" — do not imply they were.
- **OPEN, do not claim fixed (`debc39e`):** nine middle-worker defects, all failing under `LC_DEFECT_REPRO=1`. With real workers the middle stage can report READY while the endpoint is dead. Worker 1 is fixing them. The synthetic demo is unaffected.
- **`/health/ready` now reports a dead worker (`ee0b83a`)** — but the status line only re-reads it at startup and on the window `online` event (two `checkRuntime` call sites, SOURCE `main.ts:2250`, `:2275`); refreshing it after a command is **PENDING**, so a worker death shows up on the next failed command, not on the line. And a destination with no coordinates never implies you are near another shelter, while a real reservation is still recorded — browser-proved (`1271257`), SOURCE `journey.ts:447`: coordinates attach only to `FACDEMO-1`.

## (d) Fallback lines — full detail in `plan/evidence/presenter-fallback-script.md`

| State | On screen | One-line fallback |
|---|---|---|
| Backend down | `Local interface, backend not connected` | "The map and the emergency call still work; the voice command does not." |
| Workers down | line unchanged, next command: `Voice Map Control is unavailable. The displayed route and emergency call option still work.` | "The readiness probe now knows; the command is the honest signal." |
| Browser offline | `Offline, saved demo guidance` | "Saved guidance stays on screen; commands need the network." |
| Mic denied | `Microphone input stopped. Check permission or enter a command below.` | "Type the command instead — same path." |
| Autoplay blocked | no error, tap to play | "Tap Listen; autoplay is blocked by the browser." |
| Dead worker | readiness NOT_READY, line may not move until reload | "The next command will tell us; I am not going to guess." |
Never say `commandUnavailable`, `backendUnavailable` or `recognitionUnavailable` — they exist in i18n but are never rendered.

## (e) Owner's 15-minute manual checklist — before the demo, tick each box

- [ ] **Real microphone** (desktop, `127.0.0.1:18882`): record, see the transcript, get a 200. `NOT_RUN` in automation — needs a person.
- [ ] **Safari**: load the same journey end to end. `NOT_RUN` — `safaridriver` needs the Develop menu option enabled.
- [ ] **Phone over HTTPS**: the microphone needs a secure context, so plain LAN `http://` will not record — you must pick and run a tunnel yourself. **This is your decision, not mine**: it publishes an unauthenticated public URL anyone can reach. The data is synthetic, but it is still a public entry point. Vite's `allowedHosts` already lists `*.ts.net`, `.localhost.run`, `*.lhr.life`. Decide before the day, not during it.
- [ ] **200 % zoom**: real browser zoom, not `device_scale_factor` (that scales rasterisation without reflow). Walk the map, the voice console and the details dialog. `NOT_RUN` in automation.
- [ ] **Real tab switch during recording**: start recording, switch to another tab, come back. Expect the recording to be cancelled and tracking paused. `NOT_RUN` — headless Chromium cannot force `document.hidden`.
- [ ] Fallback rehearsed: kill the backend, then the workers, and say both lines out loud.
- [ ] `./tools/demo/demo.sh status` shows READY **and** the DB present; `./tools/demo/demo.sh stop` then reports the database dropped.
