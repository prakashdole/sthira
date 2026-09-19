# Assurance: safety, security, efficiency and operations

2026-09-19. This is a test plan, not a completed audit. Use a small, maintained toolset with pinned versions; do not install every overlapping scanner. Open-source license does not mean zero compute/maintenance cost.

## Layers and open-source tools

| Concern | Proposed tools / method | What they prove and miss |
| --- | --- | --- |
| Go correctness | `go test`, race detector, built-in fuzzing, `go vet` | Exercise assertions, races and parser boundaries; do not prove distributed storage correctness alone |
| Static code quality | [Staticcheck](https://staticcheck.dev/docs/); optional golangci-lint as runner | Finds many Go bugs/simplifications; avoid redundant linters and cosmetic score chasing |
| Known vulnerable dependencies | [govulncheck](https://go.dev/doc/security/) + [Trivy](https://trivy.dev/) for images/IaC/dependencies | Known databases and supported reachability; not unknown business-logic flaws |
| Go security patterns | [gosec](https://github.com/securego/gosec) | Suspect implementation patterns requiring human triage |
| Secrets | [Gitleaks](https://github.com/gitleaks/gitleaks) | Repo/diff/artifact scanning; never print discovered secrets in reports |
| SBOM and provenance | Trivy SBOM first; [Syft](https://github.com/anchore/syft) if coverage requires; [Cosign](https://github.com/sigstore/cosign) for artifact signing | Inventory and artifact provenance, not a security warranty |
| API/web dynamic checks | [OWASP ZAP](https://www.zaproxy.org/), authenticated OpenAPI/API scenarios | Common runtime vulnerabilities; custom authorization and capacity tests still needed |
| Adversarial exploration | [Strix](https://github.com/usestrix/strix), sandboxed own staging only | Exploit reproduction and hypotheses; model-dependent, non-deterministic, can miss vulnerabilities |
| Performance | Go benchmarks, [pprof](https://go.dev/blog/pprof), trace, benchstat; [k6](https://grafana.com/docs/k6/latest/) | Measured CPU/memory/latency/throughput under declared workloads; not universal efficiency scores |
| Database performance | PostgreSQL EXPLAIN ANALYZE, pg_stat_statements, contention/restore exercises | Query/lock/I/O causes; indexes justified by representative queries |
| Mobile reliability | Platform profilers/tests, Android Perfetto and Instruments on iOS; physical devices | OS-specific memory/energy/startup/thermal evidence; iOS tooling is not wholly open-source |
| Accessibility | TalkBack/VoiceOver/manual tasks; applicable WCAG/GIGW and Android accessibility checks | Native experience requires real users; web axe checks cannot certify native apps |
| AI correctness | Versioned multilingual corpus, deterministic output validator tests, human language review | Intent/entity/action reliability and injection rejection for tested cases; no universal model safety claim |

Staticcheck plus pprof/benchstat/k6 is the practical open-source answer to code-quality and efficiency checking. A JetBrains-style inspection product cannot replace profiling; security, maintainability and measured speed are separate dimensions. Use [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/), [API Security Top 10](https://owasp.org/www-project-api-security/) and [MASVS](https://mas.owasp.org/MASVS/) as coverage frameworks, not badges awarded by one scan.

## Threat model and government boundary

Threats: malicious citizen/client, stolen session, cross-district operator, compromised source account, hostile CAP/XML/GeoJSON, replayed/expired package, exposed signing keys, prompt injection in place/source text, audio decoding abuse, GPU exhaustion, capacity race, malicious dependency/model artifact and privileged audit tampering.

Controls and proof:

- Government credentials only in restricted ingestion; least privilege/read-only scope, fixed domains/endpoints, egress restrictions, redirect/IP/DNS checks, request quota and no public arbitrary proxy. Test SSRF to metadata/loopback/private ranges using local controlled fixtures, not actual government infrastructure.
- Operator MFA/roles and jurisdiction scope; citizen object-level authorization for every reservation/event. Test stolen/expired/cross-session tokens, cross-state IDs and direct endpoint access.
- Signed source packages with key rotation/revocation, rollback protection and per-source authority checks. A bad signature/digest/key/version never publishes. Check cold device bootstrap and cached trust changes.
- Bounded XML/JSON/geometry/audio decoding, no executable source content, protected parser/model processes, encrypted transport/storage, secret rotation and artifact provenance.
- Model has no government credentials, arbitrary network, database write tool or filesystem shell. Validate semantic constraints even after valid JSON generation. User and retrieved text cannot change the tool allow-list or trusted context.
- Partition resource budgets across public data, writes, ingestion and inference. Test slow clients, queue floods, retry storms and malicious large payloads. Rate limits must account for shared village/carrier NATs; IP-only denial cannot lock out all residents.
- Minimize location/audio/transcript retention; test logs, traces, temp files, backups and crash reports for leakage. Review lawful basis and applicable Indian privacy/security obligations with a qualified owner; do not hard-code unverified legal retention assumptions.

## Strix execution gate

Run after the backend flow is functional at P7; repeat relevant coverage on the integrated mobile/API release at P9. Pin repository/version, verify license and configured model, cap tokens/runtime/spend, and supply only an isolated staging target with synthetic accounts/data. Open-source Strix currently documents Docker plus an LLM/key workflow; external inference may cost money and expose test content, so select a reviewed local/hosted model explicitly.

Define allowed hosts/ports and attack categories, block outbound government/production networks, remove real secrets, snapshot disposable data and have a stop switch. Never point it at government APIs because those APIs are referenced by the app. Save redacted reports and independently reproduce findings. Fix at the responsible boundary and add deterministic regression tests; rerun affected checks. Automated reports do not replace human threat-model review or authorize live penetration testing.

## Required scenario coverage

1. Invalid/replayed/update-before-original/cancelled CAP; invalid signatures; source conflict; partial outage; malformed geometry and unsupported CRS.
2. Same village name across districts, absent location permission, wrong-language/mixed-language/noisy/clipped audio, child/older-adult speakers and code-switching. No invented confidence or location.
3. Hallucinated ID/coordinates, malicious imported instruction, excess actions/tokens, stale model response, prompt-injected tool/credential request, model timeout/cancel and GPU outage. All unsafe outputs cause zero consequential action.
4. Last available spaces with competing processes, duplicate requests/retries, expired hold racing arrival, crash after commit before response, changed payload under same key, correction below occupancy, multi-day overlap, transfer rollback and database failover.
5. No approved route, route closure while travelling, inaccessible path, road surface unknown, disconnected road graph and operator conflict. Candidate geometry must never appear as verified operational navigation.
6. Offline clean boot with/without a pack, cache eviction, package interrupted/corrupted/expired/revoked, wrong clock, stale pending write, map/audio failure, reconnect and language fallback.
7. Android+iPhone microphone/location/notification denial, incoming call/audio interruption, background suspension, low memory/storage, battery/thermal pressure, large text, screen readers and deliberate no-animation mode.
8. Normal/surge/cold-cache traffic, single-facility hotspot, source outage, queue saturation, node loss, restore and rolling upgrade. Gov upstream traffic remains within agreed quota independent of citizen count.

## Evidence and pass criteria

Record commit, build/model/data hashes, tool versions, hardware, network profile, inputs, pass/fail, p50/p95/p99 latency, error rate, throughput, resource peaks, cache ratio, query contention, voice language success and cost. Warm/cold results stay separate. No averaged language score can hide a failing regional cohort. Finite “zero failures observed” is not a promise of zero future failures.

Critical invariants (no unauthorized mutation, no fabricated operational guidance, no overbooking/replay corruption) must pass. No unresolved critical/high exploitable issue at release; lower-risk findings need a named owner and time-bounded disposition, not blanket suppression. Fixes require rerun on the final revision. Supply-chain exceptions need evidence rather than blindly accepting scanner severity.

Gate B requires P0–P7 evidence and budgeted capacity plan. Gate S additionally requires physical-device, language/accessibility, distribution, restore, security and whole-system evidence from P8–P10. Government source authorization and route/capacity ownership are separately visible gate L dependencies. If hardware, content reviewers or infrastructure are absent, mark the relevant gate blocked; do not call the software ready anyway.
