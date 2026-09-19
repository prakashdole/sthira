# Sthira delivery plan

Updated 2026-09-19. **Planning baseline; no Go migration or production-readiness claim.** This revision implements the user's production-preparation pivot and subsequent answers. It supersedes the old hackathon roadmap. Historical work stays in Git and [changes.md](changes.md), not the active backlog.

## Outcome and limits

Build an emergency relocation service for **immediate evacuation and temporary stays of 7–30 days**, with Android and iPhone available at launch. Government/authorized incident operators supply operational zones, facilities, capacity and instructions. Sthira neither predicts hazards/red zones nor identifies safe land. Permanent relocation, acquisition, rehabilitation schemes and site scoring are excluded.

Voice is the primary interaction; a visible, labelled chat control and simple touch controls remain usable without voice. Local app assets, downloaded maps, approved instructions and emergency cards survive network loss. ASR, the middle language model and TTS run on servers. Do not promise offline AI or current capacity without connectivity.

Target **one million total users**, not one million simultaneous model sessions. Estimate and benchmark disaster-time peaks using [parameters.md](parameters.md) and [equations.md](equations.md). Launch covers 10–15 demo states and their regional languages; the proposed 15-state matrix is in [feature.md](feature.md). The user supplies demo red/relocation zones after planning. Historical event evidence never makes a synthetic shelter or route authoritative.

## Decisions already made

- Go for product/backend services. Port required behavior, not all 40,000 lines.
- Android **and iPhone** from launch. Frontend framework remains undecided until backend gate B passes; Kotlin alone is not an iPhone UI decision.
- Three-stage speech pipeline, with a constrained middle model and server-side validation.
- No permanent relocation or independent hazard/safe-land prediction.
- Route authority remains **OPEN**. Navigation activation stays blocked until approved ownership and route evidence exist; synthetic route exercises can proceed.
- Only planning Markdown changes in this task. Code/file retirement is specified in [cleanup.md](cleanup.md), not executed here.

## Reading map

| Document | Owns |
| --- | --- |
| [prd.md](prd.md), [feature.md](feature.md) | Product behavior, regional demo/language scope, journeys |
| [rules.md](rules.md) | Invariants and authority boundaries |
| [tech-stack.md](tech-stack.md) | Current and proposed stack; selection gates |
| [architecture.md](architecture.md), [trd.md](trd.md) | Components, maps/routing, contracts and migration |
| [parameters.md](parameters.md), [equations.md](equations.md) | Budgets, capacity arithmetic and tunable policies |
| [decisions.md](decisions.md), [open-decisions.md](open-decisions.md) | Accepted decisions versus unresolved choices |
| [source-register.md](source-register.md) | Government activation, software/model sources and research limitations |
| [phases.md](phases.md), [prompt.md](prompt.md) | Sequencing, executable prompts, verification ledger |
| [voice-map-system-prompt.md](voice-map-system-prompt.md) | Proposed middle-model contract and examples |
| [assurance.md](assurance.md) | Security, efficiency, reliability, privacy and usability gates |
| [cleanup.md](cleanup.md) | Repository inventory, preservation and deletion prerequisites |
| [changes.md](changes.md), [memory.md](memory.md) | Historical evidence and concise resumption context |

## Delivery gates

1. **B — backend handoff:** phases P0–P7 pass: Go domain/API/storage, offline delivery, regional speech/model evaluation, security and representative load. This replaces the vague “80–90% complete.” It is a dependency gate, not an LOC percentage.
2. **S — software ready for integration:** P8–P10 pass on both real mobile platforms: accessible UI, regional language reviews, offline maps, store/distribution readiness, deployment, restore drills, final security and measured scale. Only government-dependent integration/operational approvals may remain. Missing internal work means `NOT_READY`, not “only APIs left.”
3. **L — operational release:** P11–P12 require authorized sources, route/capacity policy, working live samples, shadow exercises and accountable operational sign-off. Government integration includes data quality and field validation, not just adding an API key.

No scanner, language choice or plan guarantees a flawless evacuation. “Ready” means the stated evidence passes, known limitations are disclosed, and unsafe or uncertain guidance is withheld.

## Immediate next action

Use the P0 prompt in [prompt.md](prompt.md). Confirm the repository snapshot and public contracts, reconcile old standing instructions, and freeze the cleanup manifest. Do not start UI implementation or live government integration. Unresolved cost, device-OS and route-owner choices remain in [open-decisions.md](open-decisions.md).
