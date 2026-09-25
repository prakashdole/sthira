# Lessons

- Never substitute invented building boxes for requested real-world 3D mapping. Validate a provider's TileJSON URL, zoom limits, and decoded features before declaring coverage absent. Distinguish elevation, extruded footprints, and textured photogrammetry in implementation claims.

- A disaster interface must distinguish government authority from software presentation at every screen and API boundary.
- “God's view” should be implemented and named as Voice Map Control; avoid language suggesting omniscience or predictive certainty.
- Voice transcription is probabilistic. Deterministic intent parsing and confirmation boundaries keep it from becoming an authority path.
- Route safety cannot be inferred from ordinary road geometry; use incident-approved government routes.
- Capacity needs party size, reservation semantics, idempotency, concurrency control, expiry, and correction—not a simple mutable counter.
- A citizen reaching a map point is not proof of safe arrival. Baseline uses explicit confirmation; geofencing remains future work.
- Maps, speech, and animation can all fail during an emergency. Text-first and non-map paths are product requirements.
- ASL and ISL are different languages. For an Indian pilot, provide reviewed ISL media and never call translated text “sign language.”
- Official catalog discovery is not operational API readiness; prove access, coverage, freshness, permission, and failure behavior.
- Open-source models can satisfy a government-data-only policy when self-hosted, but their outputs are still AI-generated interface artifacts.
- A mobile emergency interface must make language and location setup explicit before guidance begins, while always keeping manual, text-first, and permission-denied paths available.
- An emergency interface cannot stop at functional controls: hierarchy, material, and typography must make the one urgent action unmistakable without turning the rest of the screen into dashboard clutter.
- Persisting first-use completion can erase safety-critical language and location choices from demonstrations and fresh emergency sessions. Keep that setup visible when the product flow requires it.
- Never place a persistent floating action over route instructions or a voice panel. A thumb-zone control must yield when a task-specific sheet is open.
- A mobile emergency map must reserve the map viewport. Default controls should collapse behind deliberate entry points, and status must not become a second large overlay.
- Decluttering must move guidance into a clearly reachable second screen, not remove the guidance a person needs to act.
- Visual cleanup must start from shared control and sheet rules. Per-screen styling creates inconsistent emergency affordances even when each screen looks acceptable alone.
- Shared components still require semantic hierarchy: emergency confirmation, route guidance, and voice control should share a grid and control grammar while visibly communicating different stakes.
- A full-page route view is navigation, not a modal. Do not borrow modal accent treatment for it; reserve one rounded, accented chrome for transient sheets and confirmations.
- Map presence needs clear semantic separation: a device dot may animate for orientation, while hazard and relocation overlays use distinct motion and color without implying live predictions.
- Voice-first CTAs need an explicit icon/text layout at narrow widths; emergency escalation needs its own high-contrast, fixed placement rather than sharing ordinary map-control weight.
- Voice emphasis can be vivid, but its accent must remain semantically separate from the red emergency-call path and leave breathing room from onboarding copy.
- Do not force a bright novelty color into a safety-critical primary control; a calm, high-contrast neutral can carry voice focus while red retains emergency meaning.
