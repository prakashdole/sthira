# Sthira demo bring-up

```sh
./tools/demo/demo.sh start    # build, DB, mock workers + exercise + Vite, wait READY
./tools/demo/demo.sh status   # per-component pid/port/URL, database, /health/ready
./tools/demo/demo.sh stop     # kill only this run's pids, drop only its database
```

Ports (127.0.0.1 only): mock workers 18880 (`DEMO_WORKER_PORT`), exercise 18881
(`DEMO_BACKEND_PORT`), Vite 18882 (`DEMO_UI_PORT`). Also `DEMO_SCENARIO`
(default `destination-choice`), `DEMO_DIR` (`/tmp/sthira-demo`),
`DEMO_READY_TIMEOUT_SECS` (60), `DEMO_ADMIN_DSN` (else `STHIRA_TEST_ADMIN_DSN`).

State and logs in `/tmp/sthira-demo/run/`: `state.env` (pids, db name,
`started_at`; never a token or DSN) + `sthira-exercise.log`, `mock-workers.log`,
`vite.log`; binaries in `/tmp/sthira-demo/bin/`. `stop` is idempotent: run 2 is a no-op, exit 0.

Scenarios (`DEMO_SCENARIO`): `default`, `silent-zoom`, `destination-choice`, `arrival-confirm`, `clarify`, `data-unavailable`, `worker-failure`.

**The mock workers play fixed scripts and do not understand speech** — nothing
you say is transcribed or understood. Real voice needs
`./tools/demo/demo.sh start --real-workers` with `STHIRA_ASR_URL`,
`STHIRA_MIDDLE_URL`, `STHIRA_TTS_URL` (optional `STHIRA_*_TOKEN`) exported in
your shell; tokens come from your shell only, never printed, logged or stored.

**Phone testing:** the browser microphone needs a secure context, so plain LAN
`http://` will not record. You must pick and run a tunnel yourself (Vite's
`allowedHosts` already lists `*.ts.net`, `.localhost.run`, `*.lhr.life`).
Exposing the demo means an **unauthenticated public URL anyone can reach** — the
data is synthetic demo data, but it is still a public entry point. Weigh that.
