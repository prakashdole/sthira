# Sthira Agent Standing Instructions & Memory (GEMINI.md)

This file contains persistent standing instructions for the coding agent. It must be respected across all turns and tasks.

## 1. Standing Agent Rules

1. **Log All Decisions in `decisions.md`**:
   - Every architectural, technical, policy, parameter, or dependency decision taken during implementation MUST be recorded as an ADR entry in `decisions.md`.
   - Entries must record: ID, Title, Status, Context, Decision, Rationale, Rejected Alternatives, Consequences, and Review Trigger.

2. **Commit Code at Every Stage**:
   - Every phase, package, spike, and logical milestone MUST be committed to git immediately upon completion.
   - Use clean, conventional commit messages (e.g., `feat(contracts): implement PKG-0C shared domain schemas and errors`, `feat(auth): add ARC-C01 authorization and compliance context`).
   - This ensures the user can review granular diffs and commit history at every stage.

3. **Strict Domain & Normative Rules Compliance**:
   - Strictly enforce all 83 rules in `rules.md` (RUL-001 through RUL-083).
   - Advisory nature of outputs (RUL-001); no automatic gazetting, title clearance, forced relocation, or authority bypass.
   - Hard gates return only `PASS`, `FAIL`, `UNKNOWN`, or `BLOCKED` (RUL-029). Missing evidence is `UNKNOWN`/`BLOCKED`, never `PASS` (RUL-013).
   - Separate dimensions for hazard, legal readiness, lean-season water, infrastructure, accessibility, livelihood, capacity, cost, and implementation readiness (RUL-028).
   - No opaque `Ω` master score (RUL-035).

4. **Ponytail Philosophy (Simplicity & Zero Bloat)**:
   - Enforce the Ponytail ladder: `YAGNI -> stdlib -> native -> one line -> minimum`.
   - Build what is specified without speculative abstractions or premature microservices.
   - Use Python standard library and clean FastAPI + Pydantic + PostGIS/Alembic where appropriate; Next.js + MapLibre GL for frontend.

5. **Web Scraping & Deep Research**:
   - For web data gathering and research, utilize available MCPs and tools: Firecrawl, Agent Reach, Scrapling, and search_web.
   - Deeply verify primary sources (Gazette, acts, government orders, CGWB, GSI, KSDMA) before asserting operational facts.

6. **Bitemporal & Audit Invariants**:
   - Facts must record `valid_time` and `system_time`.
   - Audit logs are append-only and tamper-evident with SHA-256 hash chaining.
